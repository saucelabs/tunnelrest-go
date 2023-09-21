package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/saucelabs/tunnelrest-go/region"
	assertLib "github.com/stretchr/testify/assert"
)

const (
	tunnelStateJSON = `{
		"shared_tunnel": true,
		"host": "temaki12345",
		"id": "709b9c76afee3bfef42f1a9baaa5002abf6b00a9",
		"domain_names": ["sauce-connect.proxy"],
		"status": "running",
		"tunnel_identifier": "sauce",
		"is_ready": true,
		"user_shutdown": false
	}`

	listSharedTunnelsJSON = `{"userA": [{
		"shared_tunnel": true,
		"id": "709b9c76afee3bfef42f1a9baaa5002abf6b00a9",
		"status": "running",
		"tunnel_identifier": "sauce",
		"is_ready": true,
		"user_shutdown": false
	}]}`

	allTunnelsJSON = `{"tunnels": [{
		"id": "709b9c76afee3bfef42f1a9baaa5002abf6b00a9",
		"status": "running",
		"tunnel_identifier": "sauce",
		"is_ready": true,
		"user_shutdown": false
	}]}`

	statusRunningJSON       = `{"status": "running", "is_ready": true, "user_shutdown": null, "host": "HOSTNAME", "ip_address": "1.2.3.4"}`
	statusRunningNoIPJSON   = `{"status": "running", "is_ready": true, "user_shutdown": null, "host": "HOSTNAME"}`
	statusRunningNullIPJSON = `{"status": "running", "is_ready": true, "user_shutdown": null, "host": "HOSTNAME", "ip_address": null}`
	tunID                   = "709b9c76afee3bfef42f1a9baaa5002abf6b00a9"
	tunnelRegion            = "us-west"
	tunnelName              = "my-tunnel"
	tunnelUser              = "sah"
	otherUsername           = "someotherusername"
	clientHost              = "linux-arm64"
	clientVersion           = "4.7.1"
)

var tunnelsJSON = fmt.Sprintf("[%s]", tunnelStateJSON)

// ClientConfiguration struct is used for SCUpdates response.
// Here, in tests, it's also used as a request param "configuration"
// because the value is base64-encoded and transparent to the client.
var scConfiguration = ClientConfiguration{
	Experimental: []string{"http2", "proxy"},
	Regions: []region.Region{
		{Name: "us-west", URL: "https://api.us-west-1.saucelabs.com/rest/v1"},
	},
	JobWaitTimeout:       300,
	ClientStatusInterval: 30,
	ClientStatusTimeout:  15,
	KGPHandshakeTimeout:  15,
	MaxMissedAcks:        300,
	ScproxyWriteLimit:    100,
	ScproxyReadLimit:     100,
	ServerStatusInterval: 10,
	ServerStatusTimeout:  5,
	StartTimeout:         45,
}

var updatesResponse = SCUpdates{
	SCMessages{
		Info: []string{"Lorem ipsum dolor sit amet", "consectetur adipiscing elit"},
		Warning: []string{
			"Linux32 will not be supported in the next version",
			"Your client (4.6.3) is outdated and you are using linux32!!!",
			"Download new client from https://saucelabs.com/downloads/sc-5.5.5-linux.tar.gz",
		},
	},
	scConfiguration,
}

// Helper type to make declarations shorter.
type R func(http.ResponseWriter, *http.Request)

// Helper type to make HTTP client testing easier.
type resp struct {
	handler func(http.ResponseWriter, *http.Request)
	method  string
	path    string
}

func loadTestData(t *testing.T, filename string) string {
	t.Helper()
	filePath := filepath.Join("testdata", filename)
	data, err := os.ReadFile(filePath)
	assertLib.NoErrorf(t, err, "Failed to load %s from %s", filename, filePath)
	return string(data)
}

func createTunnel(url string) (*Client, TunnelStateWithMessages, error) {
	return createTunnelWithTime(url, 1*time.Second)
}

func createTunnelWithTime(url string, timeout time.Duration) (*Client, TunnelStateWithMessages, error) {
	client := &Client{
		BaseURL: url,
		User:    tunnelUser,
		APIKey:  "password",
	}

	request := CreateTunnelRequestV4{
		DomainNames: []string{"sauce-connect.proxy"},
	}

	tunnel, err := client.CreateTunnelV4(context.Background(), &request, timeout)
	return client, tunnel, err
}

func stringResponse(s string, sc int) R {
	return func(r http.ResponseWriter, q *http.Request) {
		body := s
		if sc > 0 {
			r.WriteHeader(sc)
		}

		if s == "" {
			// Only user-agent header is used right now.
			// Will refactor the helper if needed.
			body = fmt.Sprintf(`{"user-agent": "%s"}`, q.Header.Get("user-agent"))
		}

		_, _ = io.WriteString(r, body)
	}
}

func errorResponse(code int, s string) R {
	return func(r http.ResponseWriter, _ *http.Request) {
		http.Error(r, s, code)
	}
}

// Return each response one after another. After reaching the last one,
// keep repeating it.
func multiResponseServer(responses []resp) *httptest.Server {
	index := 0

	return httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			rsp := responses[len(responses)-1]
			if index < len(responses) {
				rsp = responses[index]
				index++
			}
			if req.Method != rsp.method {
				http.Error(w,
					fmt.Sprintf("The request method %s doesn't match the expected %s", req.Method, rsp.method),
					http.StatusBadRequest)
				return
			}
			if req.URL.Path != rsp.path {
				http.Error(w,
					fmt.Sprintf("The request path %s doesn't match the expected %s", req.URL.Path, rsp.path),
					http.StatusBadRequest)
				return
			}
			rsp.handler(w, req)
		}),
	)
}

func TestTunnelState(t *testing.T) {
	assert := assertLib.New(t)

	server := multiResponseServer([]resp{
		{
			handler: stringResponse(tunnelStateJSON, http.StatusOK),
			path:    fmt.Sprintf("/%s/tunnels/%s", tunnelUser, tunID),
			method:  http.MethodGet,
		},
	})

	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		APIKey:  "password",
		User:    tunnelUser,
	}

	serverInfo, err := client.TunnelState(context.Background(), tunID)
	assert.NoErrorf(err, "client.ServerInfo errored %+v\n", err)

	expected := TunnelState{}

	err = json.Unmarshal([]byte(tunnelStateJSON), &expected)
	assert.NoErrorf(err, "Failed to unmashal %s\n", err)

	if !reflect.DeepEqual(serverInfo, expected) {
		t.Errorf("client.ServerInfo returned %+v\n", serverInfo)
	}
}

func TestTunnelStateShutdownReason(t *testing.T) {
	assert := assertLib.New(t)
	tunnelJSON := fmt.Sprintf(`{
		"domain_names": ["sauce-connect.proxy"],
		"shared_tunnel": false,
		"id": "%s",
		"status": "terminated",
		"is_ready": false,
		"tunnel_identifier": "sauce",
		"shutdown_reason": "webui",
		"user_shutdown": true
	}`, tunID)

	server := multiResponseServer([]resp{
		{
			handler: stringResponse(tunnelJSON, http.StatusOK),
			path:    fmt.Sprintf("/%s/tunnels/%s", tunnelUser, tunID),
			method:  http.MethodGet,
		},
	})

	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		APIKey:  "password",
		User:    tunnelUser,
	}

	serverInfo, err := client.TunnelState(context.Background(), tunID)
	assert.NoErrorf(err, "client.ServerInfo errored %+v\n", err)
	assert.Equalf("webui", serverInfo.ShutdownReason, "Unexpected shutdown reason in %+v\n", serverInfo)
	assert.False(serverInfo.IsReady, "Unexpected IsReady in %+v\n", serverInfo)
}

func TestListTunnels(t *testing.T) {
	assert := assertLib.New(t)

	tt := []struct {
		name      string
		otherUser string
		path      string
		client    Client
		protos    []Protocol
	}{
		{
			name: "Get kgp user tunnels",
			path: fmt.Sprintf("/%s/tunnels", tunnelUser),
			client: Client{
				UserAgent: "test",
				APIKey:    "password",
				User:      tunnelUser,
			},
			protos: []Protocol{KGPProtocol},
		},
		{
			name: "Get kgp other user tunnels",
			path: fmt.Sprintf("/%s/tunnels", otherUsername),
			client: Client{
				UserAgent:   "test",
				APIKey:      "password",
				User:        tunnelUser,
				TunnelOwner: otherUsername,
			},
			protos: []Protocol{KGPProtocol},
		},
		{
			name: "Get kgp and h2c user tunnels",
			path: fmt.Sprintf("/%s/tunnels", tunnelUser),
			client: Client{
				UserAgent: "test",
				APIKey:    "password",
				User:      tunnelUser,
			},
			protos: []Protocol{KGPProtocol, H2CProtocol},
		},
		{
			name: "Get all user tunnels",
			path: fmt.Sprintf("/%s/tunnels", tunnelUser),
			client: Client{
				UserAgent: "test",
				APIKey:    "password",
				User:      tunnelUser,
			},
			protos: []Protocol{},
		},
	}

	for _, tc := range tt {
		server := multiResponseServer([]resp{
			{
				handler: stringResponse(tunnelsJSON, http.StatusOK),
				path:    tc.path,
				method:  http.MethodGet,
			},
		})
		defer server.Close()
		tc.client.BaseURL = server.URL
		ids, err := tc.client.ListTunnels()
		assert.NoErrorf(err, "client.ListTunnels errored %+v\n", err)
		if !reflect.DeepEqual(ids, []string{tunID}) {
			t.Errorf("client.ListTunnels returned %+v\n", ids)
		}
	}
}

func TestListAllTunnelStates(t *testing.T) {
	assert := assertLib.New(t)
	server := multiResponseServer([]resp{
		{
			handler: stringResponse(allTunnelsJSON, http.StatusOK),
			path:    fmt.Sprintf("/%s/all_tunnels", tunnelUser),
			method:  http.MethodGet,
		},
	})

	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		APIKey:  "password",
		User:    tunnelUser,
	}

	limit := 1
	tunnels, err := client.ListAllTunnelStates(limit)
	assert.NoErrorf(err, "client.ListAllTunnelStates errored %+v\n", err)

	assert.Equalf(limit, len(tunnels),
		"ListAllTunnelStates unexpected response len: %+v\n", tunnels)

	assert.Equalf(tunID, tunnels[0].ID,
		"ListAllTunnelStates response %+v doesn't contain %s\n", tunnels, tunID)
}

func TestListSharedTunnelStates(t *testing.T) {
	assert := assertLib.New(t)
	path := fmt.Sprintf("/%s/tunnels", tunnelUser)

	tt := []struct {
		name   string
		client Client
		protos []Protocol
	}{
		{
			name: "Get shared KGP tunnel states",
			client: Client{
				APIKey: "password",
				User:   tunnelUser,
			},
			protos: []Protocol{KGPProtocol},
		},
		{
			name: "Get shared H2C and KGP tunnel states",
			client: Client{
				APIKey: "password",
				User:   tunnelUser,
			},
			protos: []Protocol{H2CProtocol, KGPProtocol},
		},
		{
			name: "Get all shared tunnel states",
			client: Client{
				APIKey: "password",
				User:   tunnelUser,
			},
			protos: []Protocol{},
		},
	}

	for _, tc := range tt {
		server := multiResponseServer([]resp{
			{
				path:    path,
				handler: stringResponse(listSharedTunnelsJSON, http.StatusOK),
				method:  http.MethodGet,
			},
		})

		defer server.Close()

		tc.client.BaseURL = server.URL
		tc.client.UserAgent = "test"
		tunnels, err := tc.client.ListSharedTunnelStates(tc.protos...)
		assert.NoErrorf(err,
			"%s errored %+v\n", tc.name, err)

		userA, ok := tunnels["userA"]
		assert.Truef(ok, "%s unexpected response %+v\n", tc.name, userA)

		assert.Equalf(1, len(userA),
			"%s unexpected response len: %+v\n", tc.name, userA)

		assert.Equalf(tunID, userA[0].ID,
			"%s response %+v doesn't contain %s\n", tc.name, userA, tunID)

		assert.Equalf("running", userA[0].Status,
			"%s response %+v tunnel status is not running\n", tc.name, userA)

		assert.Truef(userA[0].SharedTunnel,
			"%s response %+v SharedTunnel: true\n", tc.name, userA)
	}
}

func TestListSharedTunnels(t *testing.T) {
	assert := assertLib.New(t)
	path := fmt.Sprintf("/%s/tunnels", tunnelUser)

	tt := []struct {
		name   string
		client Client
		protos []Protocol
	}{
		{
			name: "Get kgp shared tunnels",
			client: Client{
				APIKey: "password",
				User:   tunnelUser,
			},
			protos: []Protocol{KGPProtocol},
		},
		{
			name: "Get kgp and h2c shared tunnels",
			client: Client{
				APIKey: "password",
				User:   tunnelUser,
			},
			protos: []Protocol{KGPProtocol, H2CProtocol},
		},
		{
			name: "Get all shared tunnels",
			client: Client{
				APIKey: "password",
				User:   tunnelUser,
			},
			protos: []Protocol{},
		},
	}

	for _, tc := range tt {
		server := multiResponseServer([]resp{
			{
				path:    path,
				handler: stringResponse(listSharedTunnelsJSON, http.StatusOK),
				method:  http.MethodGet,
			},
		})

		defer server.Close()

		tc.client.BaseURL = server.URL
		tc.client.UserAgent = "test"

		tunnels, err := tc.client.ListSharedTunnels()

		assert.NoErrorf(err,
			"%s errored %+v\n", tc.name, err)

		userA, ok := tunnels["userA"]
		assert.Truef(ok, "%s unexpected response %+v\n", tc.name, userA)

		assert.Equalf(1, len(userA),
			"%s unexpected response len: %+v\n", tc.name, userA)

		assert.Equalf(tunID, userA[0],
			"%s unexpected response content %+v\n", tc.name, userA)
	}
}

func TestListVPNProxiesForUser(t *testing.T) {
	const tunnelsJSON = `[{
		"id": "709b9c76afee3bfef42f1a9baaa5002abf6b00a9",
		"shared_tunnel": false,
		"status": "running",
		"tunnel_identifier": "sauce",
		"user_shutdown": false
	}]`
	path := fmt.Sprintf("/%s/tunnels", otherUsername)

	server := multiResponseServer([]resp{
		{
			path:    path,
			handler: stringResponse(tunnelsJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})
	defer server.Close()

	client := &Client{
		BaseURL:     server.URL,
		UserAgent:   "test",
		APIKey:      "password",
		TunnelOwner: otherUsername,
		User:        tunnelUser,
	}

	ids, err := client.ListVPNProxies()
	if err != nil {
		t.Errorf("client.ListVPNProxies errored %+v\n", err)
	}

	if !reflect.DeepEqual(ids, []string{tunID}) {
		t.Errorf("client.ListVPNProxies returned %+v\n", ids)
	}
}

func TestListSharedVPNsForUser(t *testing.T) {
	assert := assertLib.New(t)
	sharedTunnelsJSON := fmt.Sprintf(`{"%s": [{
		"shared_tunnel": true,
		"host": "temaki12345",
		"id": "709b9c76afee3bfef42f1a9baaa5002abf6b00a9",
		"status": "running",
		"tunnel_identifier": "sauce",
		"user_shutdown": false
	}]}`, otherUsername)
	path := fmt.Sprintf("/%s/tunnels", otherUsername)
	server := multiResponseServer([]resp{
		{
			path:    path,
			handler: stringResponse(sharedTunnelsJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})
	defer server.Close()

	client := &Client{
		BaseURL:     server.URL,
		UserAgent:   "test",
		APIKey:      "password",
		TunnelOwner: otherUsername,
		User:        tunnelUser,
	}

	tunnels, err := client.ListSharedVPNs()
	assert.Equal(nil, err,
		fmt.Sprintf("ListSharedVPNs errored %+v\n", err))

	idsUser, ok := tunnels[otherUsername]
	assert.True(ok, fmt.Sprintf("ListSharedVPNStates unexpected response %+v\n", idsUser))

	assert.Equal(1, len(idsUser),
		fmt.Sprintf("ListSharedVPNStates unexpected response len: %+v\n", idsUser))

	if !reflect.DeepEqual(idsUser, []string{tunID}) {
		t.Errorf("client.ListSharedVPNs returned %+v\n", idsUser)
	}
}

func TestListSharedVPNStatesForUser(t *testing.T) {
	assert := assertLib.New(t)
	sharedTunnelsJSON := fmt.Sprintf(`{"%s": [{
		"shared_tunnel": true,
		"host": "temaki12345",
		"id": "709b9c76afee3bfef42f1a9baaa5002abf6b00a9",
		"status": "running",
		"tunnel_identifier": "sauce",
		"user_shutdown": false
	}]}`, otherUsername)
	path := fmt.Sprintf("/%s/tunnels", otherUsername)
	server := multiResponseServer([]resp{
		{
			path:    path,
			handler: stringResponse(sharedTunnelsJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})
	defer server.Close()

	client := &Client{
		BaseURL:     server.URL,
		UserAgent:   "test",
		APIKey:      "password",
		TunnelOwner: otherUsername,
		User:        tunnelUser,
	}

	tunnels, err := client.ListSharedVPNStates()
	assert.Equal(nil, err,
		fmt.Sprintf("ListSharedVPNStates errored %+v\n", err))

	tunnelsUser, ok := tunnels[otherUsername]
	assert.True(ok, fmt.Sprintf("ListSharedVPNStates unexpected response %+v\n", tunnelsUser))

	assert.Equal(1, len(tunnelsUser),
		fmt.Sprintf("ListSharedVPNStates unexpected response len: %+v\n", tunnelsUser))

	assert.Equal(tunID, tunnelsUser[0].ID,
		fmt.Sprintf("ListSharedVPNStates response %+v doesn't contain %s\n", tunnelsUser, tunID))
}

func TestListVPNStatesForUser(t *testing.T) {
	assert := assertLib.New(t)
	path := fmt.Sprintf("/%s/tunnels", otherUsername)
	server := multiResponseServer([]resp{
		{
			path:    path,
			handler: stringResponse(tunnelsJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})
	defer server.Close()

	client := &Client{
		BaseURL:     server.URL,
		UserAgent:   "test",
		APIKey:      "password",
		TunnelOwner: otherUsername,
		User:        tunnelUser,
	}

	expected := TunnelState{
		Host:             "temaki12345",
		ID:               "709b9c76afee3bfef42f1a9baaa5002abf6b00a9",
		SharedTunnel:     true,
		Status:           "running",
		TunnelIdentifier: "sauce",
	}

	states, err := client.ListVPNStates()
	assert.NoErrorf(err, "client.ListVPNStates errored %+v\n", err)

	assert.Equalf(expected.Host, states[0].Host,
		"Expected: %+v, client.ListVPNStates returned %+v\n", expected, states)
	assert.Equalf(expected.ID, states[0].ID,
		"Expected: %+v, client.ListVPNStates returned %+v\n", expected, states)
}

func TestListTunnelStatesForUser(t *testing.T) {
	assert := assertLib.New(t)
	path := fmt.Sprintf("/%s/tunnels", otherUsername)
	server := multiResponseServer([]resp{
		{
			path:    path,
			handler: stringResponse(tunnelsJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})
	defer server.Close()

	client := &Client{
		BaseURL:     server.URL,
		UserAgent:   "test",
		APIKey:      "password",
		TunnelOwner: otherUsername,
		User:        tunnelUser,
	}

	expected := TunnelState{
		Host:             "temaki12345",
		ID:               "709b9c76afee3bfef42f1a9baaa5002abf6b00a9",
		SharedTunnel:     true,
		Status:           "running",
		TunnelIdentifier: "sauce",
	}

	states, err := client.ListTunnelStates()

	assert.NoErrorf(err, "client.ListTunnelStates errored %+v\n", err)
	assert.Equalf(expected.Host, states[0].Host,
		"Expected: %+v, client.ListTunnelStates returned %+v\n", expected, states)
	assert.Equalf(expected.ID, states[0].ID,
		"Expected: %+v, client.ListTunnelStates returned %+v\n", expected, states)
}

func TestClientShutdown(t *testing.T) {
	assert := assertLib.New(t)

	jobsRunning := 0

	path := fmt.Sprintf("/%s/tunnels/%s", tunnelUser, tunID)

	server := multiResponseServer([]resp{
		{
			path:    path,
			handler: stringResponse(fmt.Sprintf("{ \"jobs_running\": %d }", jobsRunning), http.StatusOK),
			method:  http.MethodDelete,
		},
	})

	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		APIKey:  "password",
		User:    tunnelUser,
	}

	jobs, err := client.shutdown(context.Background(), tunID, "sigterm", true)

	assert.Equal(nil, err,
		fmt.Sprintf("client.shutdown errored %+v\n", err))

	assert.Equal(jobsRunning, jobs,
		fmt.Sprintf("client.shutdown unexpected response: %+v\n", jobs))
}

func TestClientShutdownTunnel(t *testing.T) {
	jobsRunning := 1
	assert := assertLib.New(t)
	path := fmt.Sprintf("/%s/tunnels/%s", tunnelUser, tunID)

	tt := []struct {
		name       string
		server     *httptest.Server
		statusCode int
	}{
		{
			name: "One running job is returned",
			server: multiResponseServer([]resp{
				{
					path:    path,
					handler: stringResponse(fmt.Sprintf("{ \"jobs_running\": %d }", jobsRunning), http.StatusOK),
					method:  http.MethodDelete,
				},
			}),
			statusCode: http.StatusOK,
		},
		{
			name: "404 response is handled",
			server: multiResponseServer([]resp{
				{
					path:    path,
					handler: errorResponse(http.StatusNotFound, "nothing to see here"),
					method:  http.MethodDelete,
				},
			}),
			statusCode: http.StatusNotFound,
		},
	}

	for _, tc := range tt {
		defer tc.server.Close()
		client := &Client{
			BaseURL: tc.server.URL,
			APIKey:  "password",
			User:    tunnelUser,
		}

		jobs, err := client.ShutdownTunnel(context.Background(), tunID, "sigterm", true)

		if tc.statusCode == http.StatusOK {
			// no errors
			assert.Equal(nil, err,
				fmt.Sprintf("client.ShutdownTunnel errored %+v", err))
			assert.Equal(jobsRunning, jobs,
				fmt.Sprintf(
					"client.ShutdownTunnel returned unexpected jobs_running value %d, expected %d",
					jobs,
					jobsRunning,
				))
		} else {
			// ClientError is expected
			assert.Error(err, "client.ShutdownTunnel error is expected")
			var clientError *ClientError
			if !errors.As(err, &clientError) {
				t.Errorf("client.ShutdownTunnel ClientError is expected, found %+v", err)
			}
			assert.Equalf(tc.statusCode, clientError.StatusCode,
				"Invalid error, got %d expected %d, %+v", clientError.StatusCode, tc.statusCode, err)
		}
	}
}

func TestClientShutdownVPN(t *testing.T) {
	assert := assertLib.New(t)
	jobsRunning := 1
	path := fmt.Sprintf("/%s/tunnels/%s", otherUsername, tunID)
	server := multiResponseServer([]resp{
		{
			path:    path,
			handler: stringResponse(fmt.Sprintf("{ \"jobs_running\": %d }", jobsRunning), http.StatusOK),
			method:  http.MethodDelete,
		},
	})

	defer server.Close()

	client := &Client{
		BaseURL:     server.URL,
		UserAgent:   "test",
		APIKey:      "password",
		TunnelOwner: otherUsername,
		User:        tunnelUser,
	}

	jobs, err := client.ShutdownVPNProxy(context.Background(), tunID, "reason", true)
	assert.Equal(nil, err,
		fmt.Sprintf("client.ShutdownVPNProxy errored %+v", err))
	assert.Equal(jobsRunning, jobs,
		fmt.Sprintf(
			"client.ShutdownVPNProxy returned unexpected jobs_running value %d, expected %d",
			jobs,
			jobsRunning,
		))
}

func TestClientCreateTunnel(t *testing.T) {
	assert := assertLib.New(t)
	server := multiResponseServer([]resp{
		{
			path:    fmt.Sprintf("/%s/tunnels", tunnelUser),
			handler: stringResponse(statusRunningJSON, http.StatusOK),
			method:  http.MethodPost,
		},
	})

	defer server.Close()

	_, _, err := createTunnel(server.URL)
	assert.NoError(err, "client.createTunnelWithTimeout errored")
}

func TestClientCreateVPN(t *testing.T) {
	server := multiResponseServer([]resp{
		{
			path:    fmt.Sprintf("/%s/tunnels", otherUsername),
			handler: stringResponse(statusRunningJSON, http.StatusOK),
			method:  http.MethodPost,
		},
		{
			path:    fmt.Sprintf("/%s/tunnels/%s", otherUsername, tunID),
			handler: stringResponse(statusRunningJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})

	defer server.Close()

	client := &Client{
		BaseURL:     server.URL,
		UserAgent:   "test",
		User:        tunnelUser,
		TunnelOwner: otherUsername,
		APIKey:      "password",
	}

	request := CreateTunnelRequestV4{
		DomainNames: []string{"sauce-connect.proxy"},
	}

	_, err := client.CreateVPNProxy(context.Background(), &request, 1*time.Second)
	if err != nil {
		t.Errorf("client.CreateVPNProxyWithTimeout errored %+v\n", err)
	}
}

func TestClientCreateNoIPReceived(t *testing.T) {
	assert := assertLib.New(t)
	server := multiResponseServer([]resp{
		{
			path:    fmt.Sprintf("/%s/tunnels", tunnelUser),
			handler: stringResponse(statusRunningJSON, http.StatusOK),
			method:  http.MethodPost,
		},
		{
			path:    fmt.Sprintf("/%s/tunnels/%s", tunnelUser, tunID),
			handler: stringResponse(statusRunningNoIPJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})

	defer server.Close()

	_, _, err := createTunnel(server.URL)
	assert.NoErrorf(err, "client.createTunnel errored %+v\n", err)
}

func TestClientCreateNullIpReceived(t *testing.T) {
	assert := assertLib.New(t)
	server := multiResponseServer([]resp{
		{
			path:    fmt.Sprintf("/%s/tunnels", tunnelUser),
			handler: stringResponse(statusRunningJSON, http.StatusOK),
			method:  http.MethodPost,
		},
		{
			path:    fmt.Sprintf("/%s/tunnels/%s", tunnelUser, tunID),
			handler: stringResponse(statusRunningNullIPJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})

	defer server.Close()

	_, _, err := createTunnel(server.URL)
	assert.NoErrorf(err, "client.createTunnel errored %+v\n", err)
}

func TestClientCreateHTTPError(t *testing.T) {
	assert := assertLib.New(t)
	expectedHTTPStatuscode := http.StatusGatewayTimeout
	server := multiResponseServer([]resp{
		{
			path:    fmt.Sprintf("/%s/tunnels", tunnelUser),
			handler: errorResponse(expectedHTTPStatuscode, "Not available"),
			method:  http.MethodPost,
		},
	})

	defer server.Close()

	_, _, err := createTunnel(server.URL)
	assert.Error(err, "client.CreateTunnelV4 error is expected")

	var clientError *ClientError
	if !errors.As(err, &clientError) {
		t.Errorf("client.CreateTunnelV4 ClientError is expected, found %+v", err)
	}
	assert.Equalf(expectedHTTPStatuscode, clientError.StatusCode,
		"Invalid error, got %d expected %d, %s", clientError.StatusCode, expectedHTTPStatuscode, err)
}

func TestClientErrorMessage(t *testing.T) {
	assert := assertLib.New(t)
	expectedHTTPStatuscode := http.StatusBadRequest
	expectedServerResponse := `{"error": "Too many active org tunnels: N+1 >= N"}`
	server := multiResponseServer([]resp{
		{
			path:    fmt.Sprintf("/%s/tunnels", tunnelUser),
			handler: errorResponse(expectedHTTPStatuscode, expectedServerResponse),
			method:  http.MethodPost,
		},
	})

	defer server.Close()

	if _, _, err := createTunnel(server.URL); err != nil {
		assert.NotEqual(nil, err,
			"Expected client.CreateTunnelV4 error is missing")
		var clientError *ClientError
		if !errors.As(err, &clientError) {
			t.Errorf("client.CreateTunnelV4 ClientError is expected, found %+v", err)
		}

		assert.Equalf(expectedHTTPStatuscode, clientError.StatusCode,
			"Invalid error, got %d, expected %d",
			clientError.StatusCode, expectedHTTPStatuscode)

		assert.Truef(strings.Contains(clientError.ServerResponse, expectedServerResponse),
			"Invalid error, got %s expected %s", clientError.ServerResponse, expectedServerResponse)
	}
}

func TestUpdateClientStatusError(t *testing.T) {
	server := multiResponseServer([]resp{
		{
			path:    fmt.Sprintf("/%s/tunnels", tunnelUser),
			handler: stringResponse(statusRunningJSON, http.StatusOK),
			method:  http.MethodPost,
		},
		{
			path:    fmt.Sprintf("/%s/tunnels/%s", tunnelUser, tunID),
			handler: stringResponse(statusRunningJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})

	client, tunnel, err := createTunnel(server.URL)
	if err != nil {
		t.Errorf("client.createTunnel errored %+v\n", err)
	}

	server.Close()

	if _, err := client.UpdateClientStatus(context.Background(), tunnel.ID, true, time.Hour, &Memory{}); err == nil {
		t.Errorf("Expected error, got %+v", err)
	}
}

func TestGetUpdatesEmpty(t *testing.T) {
	assert := assertLib.New(t)
	// Create a mock server that responds with an empty JSON message.
	server := multiResponseServer([]resp{
		{
			path:    fmt.Sprintf("/%s/tunnels/info/updates", tunnelUser),
			handler: stringResponse("{}", http.StatusOK),
			method:  http.MethodGet,
		},
	})
	defer server.Close()

	client := &Client{
		UserAgent: "test",
		User:      tunnelUser,
		BaseURL:   server.URL,
	}

	scUpdates, err := client.GetSCUpdates(context.Background(), clientHost, clientVersion, "", tunnelRegion, "", true)
	assert.Error(err, "GetSCUpdates should error, the response doesn't contain regions info")
	assert.Equalf(0, len(scUpdates.Info), "Info messages should be empty, got %s", scUpdates.Info)
	assert.Equalf(0, len(scUpdates.Warning), "Warning messages should be empty, got %s", scUpdates.Warning)
	assert.Equalf(0, len(scUpdates.Fatal), "Fatal messages should be empty, got %s", scUpdates.Fatal)
}

func TestGetUpdates(t *testing.T) {
	assert := assertLib.New(t)
	jsonConf, err := json.Marshal(scConfiguration)
	assert.NoError(err, "Failed to marshal the SC test configuration")
	encodedJSONConf := base64.URLEncoding.EncodeToString(jsonConf)
	infoJSON, err := json.Marshal(updatesResponse)
	assert.NoError(err, "Failed to marshal the updates response stub")

	// Create a mock server that responds with the example info JSON.
	server := multiResponseServer([]resp{
		{
			path:    fmt.Sprintf("/%s/tunnels/info/updates", tunnelUser),
			handler: stringResponse(string(infoJSON), http.StatusOK),
			method:  http.MethodGet,
		},
	})
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		User:    tunnelUser,
	}

	scUpdates, _ := client.GetSCUpdates(context.Background(), "linux-386", "4.6.3", encodedJSONConf, tunnelRegion, tunnelName, true)

	assert.True(reflect.DeepEqual(updatesResponse, scUpdates), "Unexpected response")
}

func TestGetVersions(t *testing.T) {
	assert := assertLib.New(t)
	clientVersion := "4.6.3"
	versionsJSON := loadTestData(t, "versions.json")

	// Create a mock server that responds with the example info JSON.
	server := multiResponseServer([]resp{
		{
			path:    "/public/tunnels/info/versions",
			handler: stringResponse(versionsJSON, http.StatusOK),
			method:  http.MethodGet,
		},
	})
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
		User:    tunnelUser,
	}

	scVersions, err := client.GetVersions("linux-386", clientVersion, true)
	assert.NoErrorf(err,
		"Client.GetVersions errored %+v\n", err)
	assert.Equalf("4.8.0", scVersions.Latest,
		"Client.GetVersions unexpected value %+v\n", scVersions)
	assert.Equalf("UPGRADE", scVersions.Status,
		"Client.GetVersions unexpected value %+v\n", scVersions)
}

func TestRetryableCall(t *testing.T) {
	t.Skip("Skipping test that makes external calls")

	resps := []resp{}

	for _, retryableStatusCode := range RetryableStatusCodes {
		resps = append(resps, resp{
			path:    "/",
			handler: stringResponse("OK", retryableStatusCode),
			method:  http.MethodGet,
		})
	}

	// Create a mock server that responds with the example info JSON.
	server := multiResponseServer(resps)
	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
	}

	for _, retryableStatusCode := range RetryableStatusCodes {
		time.Sleep(100 * time.Millisecond)

		if err := client.executeRequest(
			context.Background(),
			http.MethodGet,
			fmt.Sprintf("https://httpbin.org/status/%d", retryableStatusCode),
			nil,
			nil,
		); err != nil {
			var cE *ClientError
			if !errors.As(err, &cE) {
				t.Errorf("client.executeRequest ClientError is expected, found %+v", err)
			}

			if cE.Retryable != true {
				t.Errorf("Retryable, got: %t expected: %t", cE.Retryable, true)
			}

			if cE.StatusCode != retryableStatusCode {
				t.Errorf("Status code, got: %d expected: %d", cE.StatusCode, retryableStatusCode)
			}
		}
	}
}

func TestNonRetryableCall(t *testing.T) {
	t.Skip("Skipping test that makes external calls")

	assert := assertLib.New(t)
	client := &Client{
		UserAgent: "test",
	}

	nonRetryableStatusCodes := []int{
		http.StatusForbidden,
		http.StatusGone,
		http.StatusTeapot,
		http.StatusLoopDetected,
	}

	for _, nonRetryableStatusCode := range nonRetryableStatusCodes {
		err := client.executeRequest(context.Background(), http.MethodGet, fmt.Sprintf("https://httpbin.org/status/%d", nonRetryableStatusCode), nil, nil)
		assert.Error(err, "Client.executeRequest error is expected")
		var cE *ClientError
		if !errors.As(err, &cE) {
			t.Errorf("client.executeRequest ClientError is expected, found %+v", err)
		}

		assert.Falsef(cE.Retryable, "Retryable, got: %t expected: %t", cE.Retryable, true)

		assert.Equalf(nonRetryableStatusCode, cE.StatusCode,
			"Status code, got: %d expected: %d", cE.StatusCode, nonRetryableStatusCode)
	}
}

func TestClientRequestHeaders(t *testing.T) {
	assert := assertLib.New(t)
	r := make(map[string]string)

	server := multiResponseServer([]resp{
		{
			handler: stringResponse("", http.StatusOK),
			path:    "/",
			method:  http.MethodGet,
		},
	})

	defer server.Close()

	client := &Client{
		BaseURL: server.URL,
	}

	err := client.executeRequest(context.Background(), http.MethodGet, server.URL, nil, &r)
	assert.NoErrorf(err, "Unexpected error received: %+v", err)
	assert.Truef(strings.EqualFold(r["user-agent"], "SauceLabs/tunnelrest-go"), "Unexpected user-agent header: %+v", r)
}
