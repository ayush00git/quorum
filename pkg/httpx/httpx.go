// Package httpx holds the JSON plumbing shared by every HTTP handler:
// strict decoding with a size cap, consistent error bodies, method checks,
// panic recovery and the optional access code.
package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

// MaxBodyBytes caps every request body.
const MaxBodyBytes = 400 << 10

// Error codes, as documented in docs/api.md.
const (
	CodeBadRequest       = "bad_request"
	CodeAccessDenied     = "access_denied"
	CodeMethodNotAllowed = "method_not_allowed"
	CodeTooLarge         = "too_large"
	CodeRateLimited      = "rate_limited"
	CodeOverloaded       = "overloaded"
	CodeUpstream         = "upstream_error"
	CodeInternal         = "internal"
	CodeNotConfigured    = "not_configured"
)

// Error is an error that knows its HTTP status and public error code.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// BadRequest builds a 400 error.
func BadRequest(format string, args ...any) *Error {
	return &Error{Status: http.StatusBadRequest, Code: CodeBadRequest, Message: fmt.Sprintf(format, args...)}
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		log.Printf("httpx: encode response: %v", err)
		WriteError(w, &Error{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "failed to encode response"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// WriteError writes the standard error body. Errors that are not *Error
// become a generic 500 so internal details never leak.
func WriteError(w http.ResponseWriter, err error) {
	var he *Error
	if !errors.As(err, &he) {
		log.Printf("httpx: internal error: %v", err)
		he = &Error{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "internal error"}
	}
	var body errorBody
	body.Error.Code = he.Code
	body.Error.Message = he.Message
	WriteJSON(w, he.Status, body)
}

// DecodeJSON reads a capped body and decodes it strictly into dst: unknown
// fields and trailing data are rejected.
func DecodeJSON(r *http.Request, dst any) error {
	data, err := io.ReadAll(io.LimitReader(r.Body, MaxBodyBytes+1))
	if err != nil {
		return BadRequest("read body: %v", err)
	}
	if len(data) > MaxBodyBytes {
		return &Error{Status: http.StatusRequestEntityTooLarge, Code: CodeTooLarge, Message: fmt.Sprintf("request body exceeds %d bytes", MaxBodyBytes)}
	}
	return DecodeStrict(data, dst)
}

// DecodeStrict decodes data into dst, rejecting unknown fields and trailing data.
func DecodeStrict(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return BadRequest("request body is empty")
		}
		return BadRequest("invalid JSON: %v", err)
	}
	if dec.More() {
		return BadRequest("invalid JSON: trailing data after the top-level value")
	}
	return nil
}

// HandlerFunc produces a response value or an error for one request.
type HandlerFunc func(r *http.Request) (any, error)

// Handle runs fn behind a method check and panic recovery, then writes
// its result (200) or error as JSON.
func Handle(w http.ResponseWriter, r *http.Request, method string, fn HandlerFunc) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("httpx: panic serving %s %s: %v", r.Method, r.URL.Path, p)
			WriteError(w, &Error{Status: http.StatusInternalServerError, Code: CodeInternal, Message: "internal error"})
		}
	}()
	if r.Method != method {
		w.Header().Set("Allow", method)
		WriteError(w, &Error{Status: http.StatusMethodNotAllowed, Code: CodeMethodNotAllowed, Message: fmt.Sprintf("use %s", method)})
		return
	}
	v, err := fn(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, v)
}
