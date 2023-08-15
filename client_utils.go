package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
)

// Decode `reader` into the object `v`, and close `reader` after.
func decodeJSON(reader io.ReadCloser, v interface{}) error {
	defer reader.Close()

	raw := &bytes.Buffer{}

	r := io.TeeReader(reader, raw)

	if err := json.NewDecoder(r).Decode(v); err != nil {
		return fmt.Errorf("couldn't decode JSON document: %w - %s", err, raw)
	}

	return nil
}

func encodeJSON(w io.Writer, v interface{}) error {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("couldn't encode JSON document: %w", err)
	}

	return nil
}

// Generates URL by properly parsing a base URL, adding paths and query params.
// It's able to modifiy (adding) an URL, already, with paths and query params.
func generateURL(
	baseURL string,
	paths []string,
	queryParams url.Values,
) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	for _, p := range paths {
		base.Path = path.Join(base.Path, p)
	}

	finalQueryParams := base.Query()

	for k, v := range queryParams {
		if k != "" && (v != nil && v[0] != "") {
			finalQueryParams[k] = v
		}
	}

	base.RawQuery = finalQueryParams.Encode()

	return base.String(), nil
}

func tunnelStatesToIDs(states []TunnelState) []string {
	ids := make([]string, len(states))
	for i, state := range states {
		ids[i] = state.ID
	}

	return ids
}

func sharedTunnelStatesToIDs(states map[string][]TunnelState) map[string][]string {
	tunnelIDs := make(map[string][]string)

	for user, userTunnels := range states {
		ids := make([]string, len(userTunnels))
		for i, state := range userTunnels {
			ids[i] = state.ID
		}

		tunnelIDs[user] = ids
	}

	return tunnelIDs
}

func pathByProto(protocol Protocol) string {
	path := "tunnels"

	if protocol == VPNProtocol {
		path = "vpns"
	}

	return path
}
