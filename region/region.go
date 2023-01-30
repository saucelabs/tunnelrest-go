package region

import (
	"fmt"

	"github.com/saucelabs/tunnelrest-go/util"
)

// Region definition.
type Region struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// String interface implementation.
func (r Region) String() string {
	if r.Name != "" && r.URL != "" {
		return fmt.Sprintf(`"%s" @ "%s"`, r.Name, util.SanitizedRawURL(r.URL))
	}

	if r.Name != "" && r.URL == "" {
		return fmt.Sprintf(`"%s"`, r.Name)
	}

	return fmt.Sprintf(`"%s"`, util.SanitizedRawURL(r.URL))
}
