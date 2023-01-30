package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizedRawURL(t *testing.T) {
	type args struct {
		u string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Should work - valid",
			args: args{
				u: "http://localhost:8080",
			},
			want: "http://localhost:8080",
		},
		{
			name: "Should work - valid",
			args: args{
				u: "https://api.eu-central-1.saucelabs.com/rest/v1/danslovsauce/tunnels/info/updates?client_host=darwin-amd64&client_version=4.8.0-beta&configuration=123459&tunnel_name=eu-mac&tunnel_pool=true",
			},
			want: "https://api.eu-central-1.saucelabs.com/rest/v1/danslovsauce/tunnels/info/updates",
		},
		{
			name: "Should work - invalid - do nothing",
			args: args{
				u: "http://||||////localhost:8080",
			},
			want: "http://||||////localhost:8080",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizedRawURL(tt.args.u)
			assert.Equalf(t, tt.want, got, "SanitizedRawURL() = %v, want %v", got, tt.want)
		})
	}
}
