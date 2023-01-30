package rest

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"testing"
)

const (
	fullUpdatesURL = "https://saucelabs.com/rest/v1/sah/tunnels/info/updates?" +
		"client_host=linux-arm64&client_version=4.7.1&configuration=" +
		"eyJleHBlcmltZW50YWwiOlsiaHR0cDIiLCJwcm94eSJdLCJqb2Jfd2FpdF90aW1l" +
		"b3V0IjozMDAsImtncF9oYW5kc2hha2VfdGltZW91dCI6MTUsIm1heF9taXNzZWRf" +
		"YWNrcyI6MzAwLCJjbGllbnRfc3RhdHVzX2ludGVydmFsIjozMCwiY2xpZW50X3N0" +
		"YXR1c190aW1lb3V0IjoxNSwicmVnaW9ucyI6W3sibmFtZSI6InVzLXdlc3QiLCJ1" +
		"cmwiOiJodHRwczovL2FwaS51cy13ZXN0LTEuc2F1Y2VsYWJzLmNvbS9yZXN0L3Yx" +
		"In1dLCJzZXJ2ZXJfc3RhdHVzX2ludGVydmFsIjoxMCwic2VydmVyX3N0YXR1c190" +
		"aW1lb3V0Ijo1LCJzdGFydF90aW1lb3V0Ijo0NX0" +
		"%3D&region=us-west&tunnel_name=my-tunnel&tunnel_pool=true"
)

func Test_urlGenerator(t *testing.T) {
	jsonConf, err := json.Marshal(scConfiguration)
	baseURL := "https://saucelabs.com/rest/v1"

	if err != nil {
		t.Error(err)
	}

	encodedJSONConf := base64.URLEncoding.EncodeToString(jsonConf)

	type args struct {
		baseURL     string
		paths       []string
		queryParams url.Values
	}
	tests := []struct {
		name       string
		args       args
		wantUpdate string
		wantErr    bool
	}{
		{
			name: "Should work - no query params",
			args: args{
				baseURL: baseURL,
				paths:   []string{tunnelUser, "tunnels/info/updates"},
			},
			wantUpdate: "https://saucelabs.com/rest/v1/sah/tunnels/info/updates",
			wantErr:    false,
		},
		{
			name: "Should work - path with trailing",
			args: args{
				baseURL: baseURL,
				paths:   []string{tunnelUser, "/tunnels/info/updates"},
			},
			wantUpdate: "https://saucelabs.com/rest/v1/sah/tunnels/info/updates",
			wantErr:    false,
		},
		{
			name: "Should work - already with already paths",
			args: args{
				baseURL: "https://saucelabs.com/rest/v1/sah/tunnels/info/updates?asdK=asdV",
				paths:   []string{"blah"},
			},
			wantUpdate: "https://saucelabs.com/rest/v1/sah/tunnels/info/updates/blah?asdK=asdV",
			wantErr:    false,
		},
		{
			name: "Should work - already with query params",
			args: args{
				baseURL: "https://saucelabs.com/rest/v1/sah/tunnels/info/updates?asdK=asdV",
				queryParams: url.Values{
					"mehK": {"mehV"},
				},
			},
			wantUpdate: "https://saucelabs.com/rest/v1/sah/tunnels/info/updates?asdK=asdV&mehK=mehV",
			wantErr:    false,
		},
		{
			name: "Should work - already with paths, and query params",
			args: args{
				baseURL: "https://saucelabs.com/rest/v1/sah/tunnels/info/updates?asdK=asdV",
				paths:   []string{"blah"},
				queryParams: url.Values{
					"mehK": {"mehV"},
				},
			},
			wantUpdate: "https://saucelabs.com/rest/v1/sah/tunnels/info/updates/blah?asdK=asdV&mehK=mehV",
			wantErr:    false,
		},
		{
			name: "Should work - adding paths, and query params",
			args: args{
				baseURL: baseURL,
				paths:   []string{tunnelUser, "tunnels/info/updates"},
				queryParams: url.Values{
					"client_host":    {clientHost},
					"client_version": {clientVersion},
					"configuration":  {encodedJSONConf},
					"region":         {tunnelRegion},
					"tunnel_name":    {tunnelName},
					"tunnel_pool":    {strconv.FormatBool(true)},
				},
			},
			wantUpdate: fullUpdatesURL,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := generateURL(tt.args.baseURL, tt.args.paths, tt.args.queryParams)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if u != tt.wantUpdate {
				t.Errorf("generateURL() = %v, want %v", u, tt.wantUpdate)
			}
		})
	}
}
