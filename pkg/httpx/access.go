package httpx

import (
	"crypto/subtle"
	"net/http"
)

// CheckAccessCode enforces the optional access code. An empty expected
// code disables the check.
func CheckAccessCode(expected, given string) error {
	if expected == "" {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(given)) != 1 {
		return &Error{Status: http.StatusUnauthorized, Code: CodeAccessDenied, Message: "a valid access code is required"}
	}
	return nil
}
