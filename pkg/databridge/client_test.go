package databridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetInfo_WithCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/info" {
			t.Errorf("expected /info, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"edition": "Enterprise",
			"activeProvider": "IbaHD",
			"availableProviders": ["IbaHD"],
			"capabilities": {
				"supportedAggregations": ["min", "max", "avg"],
				"supportedStats": ["min", "max", "mean"],
				"supportsExactComputeOnRawWindow": false
			}
		}`))
	}))
	defer server.Close()

	info, err := NewClient(server.URL).GetInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ActiveProvider != "IbaHD" {
		t.Errorf("expected provider IbaHD, got %q", info.ActiveProvider)
	}
	if info.Capabilities == nil {
		t.Fatal("expected capabilities, got nil")
	}
	if len(info.Capabilities.SupportedAggregations) != 3 {
		t.Errorf("expected 3 aggregations, got %v", info.Capabilities.SupportedAggregations)
	}
}

func TestGetInfo_NullableCapabilitiesDegradeOpen(t *testing.T) {
	// An older DataBridge image omits "capabilities" entirely.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"edition":"Community","activeProvider":"PostgreSQL","availableProviders":["PostgreSQL"]}`))
	}))
	defer server.Close()

	info, err := NewClient(server.URL).GetInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Capabilities != nil {
		t.Errorf("expected nil capabilities (degrade-open), got %+v", info.Capabilities)
	}
}

func TestQueryRecords_NotSupportedSurfacesProblemDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{
			"type": "https://databridge/errors/validation",
			"title": "Validation failed",
			"detail": "Aggregation 'sum' is not supported. Supported: avg, max, min.",
			"status": 422,
			"code": "QueryRecords.AggregationNotSupported"
		}`))
	}))
	defer server.Close()

	_, err := NewClient(server.URL).QueryRecords(context.Background(), "db", "ds", &RecordsQuery{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var apiErr *APIError
	if !asAPIError(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsNotSupported() {
		t.Errorf("expected IsNotSupported() to be true for a 422 *NotSupported, got false (code=%q status=%d)", apiErr.Code, apiErr.StatusCode)
	}
	if apiErr.Code != "QueryRecords.AggregationNotSupported" {
		t.Errorf("expected typed code, got %q", apiErr.Code)
	}
	want := "QueryRecords.AggregationNotSupported: Aggregation 'sum' is not supported. Supported: avg, max, min."
	if apiErr.Error() != want {
		t.Errorf("expected message %q, got %q", want, apiErr.Error())
	}
}

func TestAPIError_NonProblemBodyFallsBack(t *testing.T) {
	err := newAPIError(http.StatusBadGateway, []byte("upstream exploded"))
	if err.IsNotSupported() {
		t.Error("a 502 must not be treated as NotSupported")
	}
	if got := err.Error(); got != "API error 502: upstream exploded" {
		t.Errorf("unexpected fallback message: %q", got)
	}
}

// asAPIError is a tiny errors.As wrapper kept local to avoid importing errors in
// the test's hot path expectations; it mirrors the production unwrap.
func asAPIError(err error, target **APIError) bool {
	if e, ok := err.(*APIError); ok {
		*target = e
		return true
	}
	return false
}
