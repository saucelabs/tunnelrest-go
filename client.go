package rest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/saucelabs/tunnelrest-go/util"
)

const (
	infoPath = "tunnels/info"

	httpClientDefaultTimeout    = 60 * time.Second
	httpClientTransportMaxConns = 100

	// SCProtocol is name of the protocol used by Sauce Connect Proxy.
	SCProtocol Protocol = "kgp"

	// VPNProtocol is the protocol name used by IPSec Proxy.
	VPNProtocol Protocol = "ipsec"
)

// defaultUserAgent is a default user-agent header for this client.
var defaultUserAgent = fmt.Sprintf("TunnelRest/%s", Version)

var httpClient = &http.Client{
	// Timeout for requests made by this Client. The timeout includes conn
	// time, any redirects, and reading the response body. The timer remains
	// running after Get, Head, Post, or Do return and will interrupt reading
	// of the Response.Body.
	Timeout: httpClientDefaultTimeout,

	Transport: &http.Transport{
		MaxConnsPerHost:     httpClientTransportMaxConns,
		MaxIdleConns:        httpClientTransportMaxConns,
		MaxIdleConnsPerHost: httpClientTransportMaxConns,
	},
}

// Client is the Sauce Connect Proxy REST API client. It allows you to create, query, and
// terminate tunnels.
type Client struct {
	// BaseURL is REST API URL used for Sauce Connect queries.
	BaseURL string
	// GLobalBaseURL is REST API URL used for region-independent queries.
	GlobalBaseURL string
	// Headers that are set on each request.
	Headers map[string]string
	// Password is API Key used for requests authentication with the REST API.
	Password string
	// TunnelOwnerUsername is the name or ID of the user who is a subject of the query.
	TunnelOwnerUsername string
	// Username is the name or ID of the user who executes the request.
	Username string

	// DecodeJSON is used to decode a response body.
	DecodeJSON func(reader io.ReadCloser, v interface{}) error
	// EncodeJSON is used to encode a request body.
	EncodeJSON func(writer io.Writer, v interface{}) error

	// ExecuteRequest is the method that execute HTTP request.
	// http.DefaultClient.Do is used by default.
	ExecuteRequest func(*http.Request) (*http.Response, error)
}

// Options are used to initialize the client.
type Options struct {
	// GLobalBaseURL is REST API URL used for region-independent queries.
	GlobalBaseURL string
	// Headers maps header names to header values. These headers will be added to each request.
	Headers map[string]string
	// Password is API Key used for requests authentication with the REST API.
	Password string
	// TunnelOwner is the name or ID of the user who is a subject of the query.
	TunnelOwner string
	// User is the name or ID of the user who executes the request.
	User string
}

func (c *Client) decode(reader io.ReadCloser, v interface{}) error {
	if reader == nil && v != nil {
		return ErrNullReader
	}

	if c.DecodeJSON != nil {
		return c.DecodeJSON(reader, v)
	}

	return decodeJSON(reader, v)
}

func (c *Client) encode(writer io.Writer, v interface{}) error {
	if writer == nil && v != nil {
		return ErrNullWriter
	}

	if c.EncodeJSON != nil {
		return c.EncodeJSON(writer, v)
	}

	return encodeJSON(writer, v)
}

// Execute HTTP request - with context, and return an io.ReadCloser to be
// decoded. All errors are type (`ClientError`).
func (c *Client) executeRequest(
	ctx context.Context,
	method, url string,
	request, response interface{},
) error {
	var reader io.Reader

	// Encode request JSON if needed.
	if request != nil {
		var buf bytes.Buffer

		if err := c.encode(&buf, request); err != nil {
			// If Go fails to encode JSON, an encoding error is treated as an
			// internal server error.
			return &ClientError{
				Err:        err,
				Retryable:  false,
				StatusCode: http.StatusInternalServerError,
				URL:        util.SanitizedRawURL(url),
			}
		}

		reader = &buf
	}

	// Note: The context controls the entire lifetime of a request and its
	// response: obtaining a connection, sending the request, and reading the
	// response headers and body.
	//
	// Note: It has to be less than the global HTTP client timeout.
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		// Any error here is treated as an internal server error.
		return &ClientError{
			Err:        err,
			Retryable:  false,
			StatusCode: http.StatusInternalServerError,
			URL:        util.SanitizedRawURL(url),
		}
	}

	for header, val := range c.Headers {
		req.Header.Set(header, val)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	req.SetBasicAuth(c.Username, c.Password)

	resp, err := func() (*http.Response, error) {
		// Client allows to specify a request executor. IF it's not provided,
		// client will fallback to the built-in HTTP client. It provides
		// flexibility, BUT can introduce problems, out-of-control from this
		// client. Sane default values are set @ the fallback client.
		if c.ExecuteRequest == nil {
			return httpClient.Do(req)
		}

		return c.ExecuteRequest(req)
	}()
	if err != nil {
		cE := &ClientError{
			Err:       err,
			Retryable: false,
			URL:       util.SanitizedURL(req.URL),
		}

		// Sets status, if any.
		if resp != nil && resp.StatusCode != 0 {
			cE.StatusCode = resp.StatusCode
		}

		// Timeout detection.
		if errors.Is(err, context.DeadlineExceeded) {
			cE.StatusCode = http.StatusRequestTimeout
		}

		isErrorRetryable(cE)

		return cE
	}

	defer resp.Body.Close()

	// Only 2xx is considered valid.
	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		// The server may sent something, for example {"error": "xyz"}. This
		// section tries to read that.
		//
		// Note: It's safe to read here - have no Guard, because the http Client
		// and Transport guarantee that Body is always non-nil, even on
		// responses without a body or responses with a zero-length body.
		buf := new(bytes.Buffer)

		if _, err := buf.ReadFrom(resp.Body); err != nil {
			// e.g.: "Failed to read/parse resp.Body/JSON".
			return &ClientError{
				Err:        err,
				Retryable:  false,
				StatusCode: http.StatusInternalServerError,
				URL:        util.SanitizedURL(req.URL),
			}
		}

		// Reaching here means there isn't an error itself, but FOR SOME reason
		// that may be obscure/unknown, the request did not succeeded.
		cE := &ClientError{
			Err:        ErrRequestFailed,
			Retryable:  false,
			StatusCode: resp.StatusCode,
			URL:        util.SanitizedURL(req.URL),
		}

		// Does the server have any information/reason?
		if buf.String() != "" {
			cE.ServerResponse = buf.String()
		}

		isErrorRetryable(cE)

		return cE
	}

	// Decode response if needed.
	if response != nil {
		if err := c.decode(resp.Body, response); err != nil {
			// Reaching here means that the response was received. However, the server
			// response still might be not a valid JSON.
			return &ClientError{
				Err:        err,
				Retryable:  false,
				StatusCode: http.StatusInternalServerError,
				URL:        util.SanitizedURL(req.URL),
			}
		}
	}

	return nil
}

// getTunnelOwnerUsername allows to get tunnel(s) for an arbitrary user.
func (c *Client) getTunnelOwnerUsername() string {
	if len(c.TunnelOwnerUsername) > 0 {
		return c.TunnelOwnerUsername
	}

	return c.Username
}

// listSharedTunnels returns tunnel states per user in the org with shared tunnels.
func (c *Client) listSharedTunnels(protocol Protocol) (map[string][]TunnelState, error) {
	states := make(map[string][]TunnelState)
	url := fmt.Sprintf("%s/%s/tunnels?full=1&all=1&backend=%s", c.BaseURL, c.getTunnelOwnerUsername(), protocol)

	err := c.executeRequest(context.Background(), http.MethodGet, url, nil, &states)

	return states, err
}

// listTunnels returns tunnels for a given user for a given protocol (kgp or ipsec).
func (c *Client) listTunnels(protocol Protocol) ([]TunnelState, error) {
	var states []TunnelState

	url := fmt.Sprintf("%s/%s/tunnels?full=1&backend=%s", c.BaseURL, c.getTunnelOwnerUsername(), protocol)
	err := c.executeRequest(context.Background(), http.MethodGet, url, nil, &states)

	return states, err
}

// Returns all the user tunnels (including already terminated ones).
func (c *Client) listAllTunnels(limit int) (map[string][]TunnelState, error) {
	tunnels := map[string][]TunnelState{}
	url := fmt.Sprintf("%s/%s/all_tunnels", c.BaseURL, c.getTunnelOwnerUsername())

	if limit > 0 {
		url = fmt.Sprintf("%s?limit=%d", url, limit)
	}

	err := c.executeRequest(context.Background(), http.MethodGet, url, nil, &tunnels)

	return tunnels, err
}

// Terminates Sauce Proxy. Termination `reason` could be "sigterm",
// "serverTimeout", etc... `wait` determines whether the control logic should
// wait for jobs to finish before terminating the tunnel.
func (c *Client) shutdown(ctx context.Context, id string, reason string, wait bool, protocol Protocol) (int, error) {
	u, err := generateURL(fmt.Sprintf("%s/%s",
		c.BaseURL,
		path.Join(c.getTunnelOwnerUsername(), pathByProto(protocol), id),
	), nil, url.Values{
		"reason": {reason},
	})
	if err != nil {
		return -1, err
	}

	if wait {
		u, err = generateURL(u, nil, url.Values{
			"wait": {"1"},
		})
	} else {
		u, err = generateURL(u, nil, url.Values{
			"wait": {"0"},
		})
	}

	if err != nil {
		return -1, err
	}

	var response struct {
		JobsRunning int `json:"jobs_running"`
	}

	if err := c.executeRequest(
		ctx,
		http.MethodDelete,
		u,
		nil,
		&response,
	); err != nil {
		return 0, err
	}

	return response.JobsRunning, nil
}

// create requests Sauce Labs REST API to provision a new tunnel.
func (c *Client) create(
	ctx context.Context,
	request *Request,
	protocol Protocol,
) (TunnelStateWithMessages, error) {
	req := request

	doc := jsonRequest{
		DirectDomains:    &req.DirectDomains,
		DomainNames:      req.DomainNames,
		ExtraInfo:        &req.ExtraInfo,
		FastFailRegexps:  &req.FastFailRegexps,
		Metadata:         req.Metadata,
		NoProxyCaching:   req.NoProxyCaching,
		NoSSLBumpDomains: &req.NoSSLBumpDomains,
		SharedTunnel:     req.SharedTunnel,
		SquidConfig:      nil,
		SSHPort:          req.KGPPort,
		TunnelIdentifier: &req.TunnelIdentifier,
		TunnelPool:       req.TunnelPool,
		UseKGP:           true,
		VMVersion:        &req.VMVersion,
	}

	tunnel := TunnelStateWithMessages{}

	url := fmt.Sprintf("%s/%s/%s", c.BaseURL, c.getTunnelOwnerUsername(), pathByProto(protocol))

	if err := c.executeRequest(ctx, http.MethodPost, url, doc, &tunnel); err != nil {
		return tunnel, err
	}

	return tunnel, nil
}

// GetSCUpdates retrieves user messages, and the client version/platform
// specific default configuration.
func (c *Client) GetSCUpdates(
	ctx context.Context,
	platform, version, configuration, region, tunnelName string,
	isTunnelPool bool,
) (SCUpdates, error) {
	resp := SCUpdates{}

	infoURL, err := generateURL(
		c.BaseURL,
		[]string{
			c.getTunnelOwnerUsername(),
			infoPath,
			"updates",
		},
		url.Values{
			"client_host":    {platform},
			"client_version": {version},
			"configuration":  {configuration},
			"region":         {region},
			"tunnel_name":    {tunnelName},
			"tunnel_pool":    {strconv.FormatBool(isTunnelPool)},
		},
	)
	// failed to configure URL
	if err != nil {
		return resp, err
	}

	if err := c.executeRequest(ctx, http.MethodGet, infoURL, nil, &resp); err != nil {
		return resp, err
	}

	if len(resp.Configuration.Regions) < 1 {
		return resp, MissingRegionsInformation(infoURL)
	}

	return resp, nil
}

// GetVersions retrieves Sauce Connect versions info.
func (c *Client) GetVersions(
	platform, version string,
	all bool,
) (SCVersions, error) {
	resp := SCVersions{}

	versionsURL, err := generateURL(
		c.BaseURL,
		[]string{
			"public",
			infoPath,
			"versions",
		},
		url.Values{
			"client_version": {version},
			"client_host":    {platform},
			"all":            {strconv.FormatBool(all)},
		},
	)
	// failed to configure URL
	if err != nil {
		return resp, err
	}

	if err := c.executeRequest(context.Background(), http.MethodGet, versionsURL, nil, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

// UpdateClientStatus updates Sauce Labs REST API with the client status for tunnel `id`.
func (c *Client) UpdateClientStatus(
	ctx context.Context,
	id string,
	connected bool,
	duration time.Duration,
	memory *Memory,
) (UpdateClientStatusResponse, error) {
	url := fmt.Sprintf("%s/%s/tunnels/%s/connected", c.BaseURL, c.Username, id)
	resp := UpdateClientStatusResponse{}

	req := ClientStatusRequest{
		KGPConnected:         connected,
		StatusChangeDuration: int64(duration.Seconds()),
		Memory:               memory,
	}

	if err := c.executeRequest(ctx, http.MethodPost, url, &req, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

// ReportCrash is used to update Sauce Labs REST API that client crashed.
func (c *Client) ReportCrash(tunnel, info, logs string) error {
	doc := struct {
		Info   string `json:"Info"`
		Logs   string `json:"Logs"`
		Tunnel string `json:"Tunnel"`
	}{Tunnel: tunnel, Info: info, Logs: logs}

	url := fmt.Sprintf("%s/%s/errors", c.BaseURL, c.Username)

	return c.executeRequest(context.Background(), http.MethodPost, url, doc, nil)
}

// TunnelState returns the tunnel `id` information obtained from Sauce Labs REST API.
func (c *Client) TunnelState(ctx context.Context, id string) (TunnelState, error) {
	info := TunnelState{}
	url := fmt.Sprintf("%s/%s/tunnels/%s", c.BaseURL, c.getTunnelOwnerUsername(), id)

	err := c.executeRequest(ctx, http.MethodGet, url, nil, &info)

	return info, err
}

// New returns REST API client.
func New(
	restURL string,
	options Options,
) (*Client, error) {
	headers := make(map[string]string)
	headers["user-agent"] = defaultUserAgent

	for header, val := range options.Headers {
		headers[strings.ToLower(header)] = val
	}
	// TODO: Add options validation.
	return &Client{
		BaseURL:             restURL,
		GlobalBaseURL:       options.GlobalBaseURL,
		Headers:             headers,
		Password:            options.Password,
		TunnelOwnerUsername: options.TunnelOwner,
		Username:            options.User,
	}, nil
}
