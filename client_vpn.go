package rest

import (
	"context"
	"time"
)

// CreateVPNProxy requests Sauce Labs REST API to start a new proxy over VPN.
func (c *Client) CreateVPNProxy(
	ctx context.Context, request *CreateTunnelRequestV4, timeout time.Duration,
) (TunnelStateWithMessages, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	request.Protocol = string(VPNProtocol)
	// Releases resources if the request completes before timeout elapses.
	defer cancel()

	return c.create(ctx, request)
}

// ListVPNProxies returns VPN proxy IDs for a given user.
func (c *Client) ListVPNProxies() ([]string, error) {
	states, err := c.listTunnels(VPNProtocol)
	if err != nil {
		return nil, err
	}

	return tunnelStatesToIDs(states), nil
}

// ListVPNStates returns VPN proxy states for a given user.
func (c *Client) ListVPNStates() ([]TunnelState, error) {
	return c.listTunnels(VPNProtocol)
}

// ListSharedVPNs returns proxy IDs per user for a given org with shared proxies.
func (c *Client) ListSharedVPNs() (map[string][]string, error) {
	tunnels, err := c.listSharedTunnels(VPNProtocol)
	if err != nil {
		return nil, err
	}

	return sharedTunnelStatesToIDs(tunnels), nil
}

// ListSharedVPNStates returns VPN proxy states per user for a given org with shared proxies.
func (c *Client) ListSharedVPNStates() (map[string][]TunnelState, error) {
	return c.listSharedTunnels(VPNProtocol)
}

// ShutdownVPNProxy terminates VPN proxy.
// Boolean "wait" determines whether the server
// should wait for jobs to finish.
func (c *Client) ShutdownVPNProxy(ctx context.Context, id string, reason string, wait bool) (int, error) {
	return c.shutdown(ctx, id, reason, wait)
}
