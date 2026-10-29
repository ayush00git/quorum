package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type payload struct {
	Name string `json:"name"`
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) (int, string) {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %v (%s)", err, rec.Body.String())
	}
	return rec.Code, body.Error.Code
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode string
	}{
		{"valid", `{"name":"x"}`, ""},
		{"unknown field", `{"name":"x","extra":1}`, CodeBadRequest},
		{"malformed", `{"name":`, CodeBadRequest},
		{"empty", ``, CodeBadRequest},
		{"trailing", `{"name":"x"} {}`, CodeBadRequest},
		{"too large", `{"name":"` + strings.Repeat("a", MaxBodyBytes) + `"}`, CodeTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			var p payload
			err := DecodeJSON(r, &p)
			if tc.wantCode == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var he *Error
			if !errors.As(err, &he) || he.Code != tc.wantCode {
				t.Fatalf("got %v, want code %s", err, tc.wantCode)
			}
		})
	}
}

func TestHandle(t *testing.T) {
	ok := func(*http.Request) (any, error) { return map[string]int{"n": 1}, nil }
	tests := []struct {
		name       string
		method     string
		fn         HandlerFunc
		wantStatus int
		wantCode   string
	}{
		{"ok", http.MethodGet, ok, http.StatusOK, ""},
		{"wrong method", http.MethodPost, ok, http.StatusMethodNotAllowed, CodeMethodNotAllowed},
		{"typed error", http.MethodGet, func(*http.Request) (any, error) { return nil, BadRequest("nope") }, http.StatusBadRequest, CodeBadRequest},
		{"plain error hidden", http.MethodGet, func(*http.Request) (any, error) { return nil, errors.New("secret detail") }, http.StatusInternalServerError, CodeInternal},
		{"panic recovered", http.MethodGet, func(*http.Request) (any, error) { panic("boom") }, http.StatusInternalServerError, CodeInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Handle(rec, httptest.NewRequest(tc.method, "/", nil), http.MethodGet, tc.fn)
			if tc.wantCode == "" {
				if rec.Code != tc.wantStatus {
					t.Fatalf("status %d, want %d", rec.Code, tc.wantStatus)
				}
				return
			}
			status, code := decodeError(t, rec)
			if status != tc.wantStatus || code != tc.wantCode {
				t.Fatalf("got %d %s, want %d %s", status, code, tc.wantStatus, tc.wantCode)
			}
			if strings.Contains(rec.Body.String(), "secret detail") {
				t.Fatal("internal error detail leaked")
			}
		})
	}
}

func TestCheckAccessCode(t *testing.T) {
	if err := CheckAccessCode("", "anything"); err != nil {
		t.Fatalf("disabled check failed: %v", err)
	}
	if err := CheckAccessCode("s3cret", "s3cret"); err != nil {
		t.Fatalf("matching code rejected: %v", err)
	}
	var he *Error
	if err := CheckAccessCode("s3cret", "wrong"); !errors.As(err, &he) || he.Status != http.StatusUnauthorized {
		t.Fatalf("wrong code: got %v", err)
	}
}
