package rest

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/saucelabs/tunnelrest-go/util"
)

var (
	ErrMissingRegions = errors.New(`missing "regions" information`)
	ErrNullReader     = errors.New("can't decode JSON from a null reader")
	ErrNullWriter     = errors.New("can't decode JSON from a null writer")
	ErrRequestFailed  = errors.New("HTTP request failed")
)

// RetryableErrorMessages is a list of non-HTTP status code error messages that
// should be retried.
var RetryableErrorMessages = []string{}

// RetryableStatusCodes is a list of "usually accepted" retryable HTTP status
// codes.
// For more, see:
// - https://stackoverflow.com/questions/47680711/which-http-errors-should-never-trigger-an-automatic-retry
// - https://softwareengineering.stackexchange.com/questions/382594/should-you-retry-500-api-errors
var RetryableStatusCodes = []int{
	http.StatusBadGateway,
	http.StatusConflict,
	http.StatusGatewayTimeout,
	http.StatusInternalServerError,
	http.StatusRequestTimeout,
	http.StatusServiceUnavailable,
	http.StatusTooManyRequests,
}

// ClientError definition.
type ClientError struct {
	Err error
	// Message, if provided, will be used instead of the usual message.
	Message        string
	Retryable      bool
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
		Retryable:  false,
		StatusCode: http.StatusInternalServerError,
		URL:        util.SanitizedRawURL(url),
	}
}

// isErrorRetryable returns true when the client error is retryable.
func isErrorRetryable(cE *ClientError) {
	// Should only verify if there is a cE.
	if cE == nil {
		return
	}

	//////
	// Error message introspection-based verification.
	//////

	if cE.Err != nil {
		for _, retryableErrorMessage := range RetryableErrorMessages {
			if strings.Contains(cE.Err.Error(), retryableErrorMessage) {
				cE.Retryable = true

				return
			}
		}
	}

	//////
	// HTTP status code-based verification.
	//////

	// Should only verify if a status code is defined.
	if cE.StatusCode != 0 {
		// Should only be retryable if status code is in RetryableStatusCodes list.
		for _, retryableStatusCode := range RetryableStatusCodes {
			if cE.StatusCode == retryableStatusCode {
				cE.Retryable = true
			}
		}
	}
}
