package rest

import (
	"fmt"

	"github.com/saucelabs/tunnelrest-go/region"
)

// Memory contains client host memory info.
type Memory struct {
	// Total memory in bytes.
	Total uint64 `json:"total"`
	// Available memory in bytes.
	Available uint64 `json:"available"`
	// Used memory in bytes.
	Used uint64 `json:"used"`
	// Free memory in bytes.
	Free uint64 `json:"free"`
}

type Metadata struct {
	Build         string            `json:"build"`
	Command       string            `json:"command"`
	CommandArgs   string            `json:"command_args"`
	ExternalProxy string            `json:"external_proxy,omitempty"`
	Extra         map[string]string `json:"extra,omitempty"`
	GitVersion    string            `json:"git_version"`
	Hostname      string            `json:"hostname"`
	HostCPU       string            `json:"host_cpu,omitempty"`
	HostMemory    uint64            `json:"host_memory,omitempty"`
	NoFileLimit   uint64            `json:"nofile_limit"`
	Platform      string            `json:"platform"`
	Release       string            `json:"release"`
}

//nolint:maligned
type jsonRequest struct {
	DirectDomains    *[]string `json:"direct_domains"`
	DomainNames      []string  `json:"domain_names"`
	ExtraInfo        *string   `json:"extra_info"`
	FastFailRegexps  *[]string `json:"fast_fail_regexps"`
	Metadata         Metadata  `json:"metadata"`
	NoProxyCaching   bool      `json:"no_proxy_caching"`
	NoSSLBumpDomains *[]string `json:"no_ssl_bump_domains"`
	SharedTunnel     bool      `json:"shared_tunnel"`
	SquidConfig      *string   `json:"squid_config"`
	SSHPort          int       `json:"ssh_port"`
	TunnelIdentifier *string   `json:"tunnel_identifier"`
	TunnelPool       bool      `json:"tunnel_pool"`
	UseKGP           bool      `json:"use_kgp"`
	VMVersion        *string   `json:"vm_version"`
}

// Request for a new tunnel.
//
//nolint:maligned
type Request struct {
	DomainNames      []string
	TunnelIdentifier string

	DirectDomains    []string
	FastFailRegexps  []string
	KGPPort          int
	NoProxyCaching   bool
	NoSSLBumpDomains []string
	SharedTunnel     bool
	TunnelPool       bool
	VMVersion        string

	// Metadata contains the request metadata.
	Metadata Metadata

	// Extra info. This is a string (which contains a JSON dict) to enable
	// optional features and flags.
	ExtraInfo string
}

type ClientStatusRequest struct {
	KGPConnected         bool    `json:"kgp_is_connected"`
	StatusChangeDuration int64   `json:"kgp_seconds_since_last_status_change"`
	Memory               *Memory `json:"memory,omitempty"`
}

// TunnelState contains a detailed tunnel information as returned by REST API.
type TunnelState struct {
	CreationTime     int      `json:"creation_time"`
	ShutdownTime     int      `json:"shutdown_time,omitempty"`
	ExtraInfo        string   `json:"extra_info,omitempty"`
	Host             string   `json:"host"`
	ID               string   `json:"id"`
	IP               string   `json:"ip_address,omitempty"`
	Metadata         Metadata `json:"metadata,omitempty"`
	Owner            string   `json:"owner"`
	SharedTunnel     bool     `json:"shared_tunnel"`
	IsReady          bool     `json:"is_ready"`
	ShutdownReason   string   `json:"shutdown_reason,omitempty"`
	Status           string   `json:"status"`
	TunnelIdentifier string   `json:"tunnel_identifier"`
	UserShutdown     *bool    `json:"user_shutdown,omitempty"`
}

type TunnelStateWithMessages struct {
	TunnelState
	Messages SCMessages `json:"messages,omitempty"`
}

// UpdateClientStatusResponse is the REST API response.
type UpdateClientStatusResponse struct {
	ID     string `json:"id"`
	Result bool   `json:"result"`
}

// ClientConfiguration definition.
type ClientConfiguration struct {
	Experimental         []string        `json:"experimental,omitempty"`
	JobWaitTimeout       int             `json:"job_wait_timeout,omitempty"`
	KGPHandshakeTimeout  int             `json:"kgp_handshake_timeout,omitempty"`
	MaxMissedAcks        int             `json:"max_missed_acks,omitempty"`
	ClientStatusInterval int             `json:"client_status_interval,omitempty"`
	ClientStatusTimeout  int             `json:"client_status_timeout,omitempty"`
	Regions              []region.Region `json:"regions,omitempty"`
	ServerStatusInterval int             `json:"server_status_interval,omitempty"`
	ServerStatusTimeout  int             `json:"server_status_timeout,omitempty"`
	StartTimeout         int             `json:"start_timeout,omitempty"`
}

// SCMessages contains messages that grouped by the severity level.
type SCMessages struct {
	Fatal   []string `json:"fatal,omitempty"`
	Info    []string `json:"info,omitempty"`
	Warning []string `json:"warning,omitempty"`
}

// SCUpdates contains a response from /updates endpoint.
type SCUpdates struct {
	SCMessages
	Configuration ClientConfiguration `json:"configuration"`
}

// ClientDownloadInfo contains a SC client download info.
type ClientDownloadInfo struct {
	DownloadURL string `json:"download_url"`
	SHA1        string `json:"sha1"`
}

// DownloadByPlatform contains a SC client download info per platform.
type DownloadByPlatform struct {
	Linux      ClientDownloadInfo `json:"linux"`
	LinuxARM64 ClientDownloadInfo `json:"linux-arm64,omitempty"`
	Win32      ClientDownloadInfo `json:"win32,omitempty"`
	MacOS      ClientDownloadInfo `json:"osx"`
}

// SCVersions contains a response from /versions endpoint.
type SCVersions struct {
	Latest        string                        `json:"latest_version"`
	ClientVersion string                        `json:"client_version,omitempty"`
	Status        string                        `json:"status,omitempty"`
	InfoURL       string                        `json:"info_url"`
	DownloadURL   string                        `json:"download_url"`
	SHA1          string                        `json:"sha1"`
	Warning       []string                      `json:"warning,omitempty"`
	Downloads     DownloadByPlatform            `json:"downloads"`
	AllDownloads  map[string]DownloadByPlatform `json:"all_downloads,omitempty"`
}

func (m Memory) String() string {
	return fmt.Sprintf("Total: %d, Available: %d, Used: %d, Free: %d",
		m.Total, m.Available, m.Used, m.Free)
}
