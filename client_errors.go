package rest

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/saucelabs/tunnelrest-go/util"
)

var (
	ErrMissingRegions = errors.New(`missing "regions" information`)
	ErrNullReader     = errors.New("can't decode JSON from a null reader")
	ErrNullWriter     = errors.New("can't decode JSON from a null writer")
	ErrRequestFailed  = errors.New("HTTP request failed")
)

// ClientError definition.
type ClientError struct {
	Err error
	// Message, if provided, will be used instead of the usual message.
	Message        string
	ServerResponse string
	StatusCode     int
	URL            string
}

// Error interface implementation.
func (cE *ClientError) Error() string {
	// Allows to specify a concise error message.
	if cE.Message != "" {
		return cE.Message
	}

	errMsg := fmt.Sprintf("URL %s", util.SanitizedRawURL(cE.URL))

	if cE.StatusCode != 0 {
		errMsg = fmt.Sprintf("%s - %d (%s)", errMsg, cE.StatusCode, http.StatusText(cE.StatusCode))
	}

	if cE.Err != nil {
		errMsg = fmt.Sprintf("%s Error: %s", errMsg, fmt.Errorf("%w", cE.Err))
	}

	if cE.ServerResponse != "" {
		errMsg = fmt.Sprintf("%s. Server response: %s", errMsg, cE.ServerResponse)
	}

	return errMsg
}

// Unwrap interface implementation.
func (cE *ClientError) Unwrap() error { return cE.Err }

// Short returns the HTTP status code and its text version.
func (cE *ClientError) Short() string {
	if cE.StatusCode != 0 {
		return fmt.Sprintf("%d (%s)", cE.StatusCode, http.StatusText(cE.StatusCode))
	}

	return fmt.Sprintf("Failed to reach %s", util.SanitizedRawURL(cE.URL))
}

// MissingRegionsInformation indicates that the response doesn't contain
// Sauce Labs `regions` information.
var MissingRegionsInformation = func(url string) *ClientError {
	return &ClientError{
		Err:        ErrMissingRegions,
		StatusCode: http.StatusInternalServerError,
		URL:        util.SanitizedRawURL(url),
	}
}
