package util

import (
	"fmt"
	"net/url"
)

// SanitizedURL returns the sanitized form (scheme, host, and path) of an URL.
func SanitizedURL(u *url.URL) string {
	sanitizedURL := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	if u.Path != "" {
		sanitizedURL = fmt.Sprintf("%s%s", sanitizedURL, u.Path)
	}

	return sanitizedURL
}

// SanitizedRawURL returns the sanitized form (scheme, host, and path) of a raw
// URL.
func SanitizedRawURL(u string) string {
	sanitizedURL := u

	parsedReceivedURL, err := url.Parse(u)
	if parsedReceivedURL != nil && err == nil {
		sanitizedURL = SanitizedURL(parsedReceivedURL)
	}

	return sanitizedURL
}
