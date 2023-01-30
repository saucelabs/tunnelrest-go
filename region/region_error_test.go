package region

import (
	"strings"
	"testing"
)

func TestInvalidRegionError_Error(t *testing.T) {
	someURL := "http://localhost:8080"

	type fields struct {
		AvailableRegions string
		PossibleRegion   Region
		Region           Region
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Should work - Full",
			fields: fields{
				AvailableRegions: strings.Join([]string{`"region1", "region2", "region3"`}, ", "),
				PossibleRegion:   Region{Name: "region1", URL: someURL},
				Region:           Region{Name: "regn1", URL: someURL},
			},
			want: `Unknown "regn1" @ "` + someURL + `". Did you meant "region1" @ "` + someURL + `"? Available: "region1", "region2", "region3"`,
		},
		{
			name: "Should work - Region",
			fields: fields{
				Region: Region{Name: "regn1", URL: someURL},
			},
			want: `Unknown "regn1" @ "` + someURL + `".`,
		},
		{
			name: "Should work - Region and Suggestion",
			fields: fields{
				PossibleRegion: Region{Name: "region1", URL: someURL},
				Region:         Region{Name: "regn1", URL: someURL},
			},
			want: `Unknown "regn1" @ "` + someURL + `". Did you meant "region1" @ "` + someURL + `"?`,
		},
		{
			name: "Should work - Region and Suggestion",
			fields: fields{
				AvailableRegions: strings.Join([]string{`"region1", "region2", "region3"`}, ", "),
				Region:           Region{Name: "regn1", URL: someURL},
			},
			want: `Unknown "regn1" @ "` + someURL + `". Available: "region1", "region2", "region3"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iR := &InvalidRegionError{
				Available:       tt.fields.AvailableRegions,
				PossibleRegion:  tt.fields.PossibleRegion,
				SpecifiedRegion: tt.fields.Region,
			}
			if got := iR.Error(); got != tt.want {
				t.Errorf("InvalidRegionError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}
