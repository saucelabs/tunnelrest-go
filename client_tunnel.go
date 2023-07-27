package rest

import (
	"context"
	"time"
)

// CreateTunnel requests Sauce Labs REST API to create a new tunnel.
func (c *Client) CreateTunnel(
	ctx context.Context, request *Request, timeout time.Duration,
) (TunnelStateWithMessages, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	// Releases resources if the request completes before timeout elapses.
	defer cancel()

	return c.create(ctx, request)
}

// ListAllTunnelStates returns all the tunnels (including not currently running)
// for a given user.
func (c *Client) ListAllTunnelStates(limit int) ([]TunnelState, error) {
	allTunnels, err := c.listAllTunnels(limit)
	if err != nil {
		return nil, err
	}

	tunnels, ok := allTunnels["tunnels"]

	if ok {
		return tunnels, nil
	}

	return nil, nil
}

// ListSharedTunnels returns tunnel IDs per user for a given org with shared tunnels.
// Filter results by one or more protocol, or leave empty for all protocols.
func (c *Client) ListSharedTunnels(protocol ...Protocol) (map[string][]string, error) {
	tunnels, err := c.listSharedTunnels(protocol...)
	if err != nil {
		return nil, err
	}

	return sharedTunnelStatesToIDs(tunnels), nil
}

// ListSharedTunnelStates returns tunnels per user for a given org with shared tunnels.
// Filter results by one or more protocol, or leave empty for all protocols.
func (c *Client) ListSharedTunnelStates(protocol ...Protocol) (map[string][]TunnelState, error) {
	return c.listSharedTunnels(protocol...)
}

// ListTunnels returns tunnel IDs for a given user.
// Filter results by one or more protocol, or leave empty for all protocols.
func (c *Client) ListTunnels(protocol ...Protocol) ([]string, error) {
	states, err := c.listTunnels(protocol...)
	if err != nil {
		return nil, err
	}

	return tunnelStatesToIDs(states), nil
}

// ListTunnelStates returns KGP tunnel states for a given user.
func (c *Client) ListTunnelStates(protocol ...Protocol) ([]TunnelState, error) {
	return c.listTunnels(protocol...)
}

// ShutdownTunnel terminates tunnel. Termination 'reason' could be
// "sigterm", "serverTimeout", etc... Boolean "wait" determines whether the server
// should wait for jobs to finish.
func (c *Client) ShutdownTunnel(ctx context.Context, id string, reason string, wait bool) (int, error) {
	return c.shutdown(ctx, id, reason, wait)
}
