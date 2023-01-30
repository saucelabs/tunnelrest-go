package region

import "fmt"

// InvalidRegionError throw when a region is invalid.
type InvalidRegionError struct {
	Available       string
	PossibleRegion  Region
	SpecifiedRegion Region
}

// Error interface implementation.
func (iR *InvalidRegionError) Error() string {
	errMsg := fmt.Sprintf("Unknown %s.", iR.SpecifiedRegion)

	if (iR.PossibleRegion != Region{}) {
		errMsg = fmt.Sprintf(`%s Did you meant %s?`, errMsg, iR.PossibleRegion)
	}

	if iR.Available != "" {
		errMsg = fmt.Sprintf(`%s Available: %s`, errMsg, iR.Available)
	}

	return errMsg
}
