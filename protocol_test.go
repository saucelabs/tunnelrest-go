package rest

import "testing"

func TestProtocol_String(t *testing.T) {
	var customProtocol Protocol = "customProtocol"

	tests := []struct {
		name string
		tr   *Protocol
		want string
	}{
		{
			name: "Should work",
			tr:   &customProtocol,
			want: "customProtocol",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tr.String(); got != tt.want {
				t.Errorf("Protocol.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
