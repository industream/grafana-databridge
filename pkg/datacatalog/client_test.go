package datacatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

// newPagedCatalogServer serves totalEntries catalog entries honoring limit/offset,
// like DataCatalog's GET /catalog-entries.
func newPagedCatalogServer(t *testing.T, totalEntries int, isTotalCountSent bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		items := []CatalogEntry{}
		for i := offset; i < totalEntries && i < offset+limit; i++ {
			items = append(items, CatalogEntry{ID: fmt.Sprintf("entry-%d", i), Name: fmt.Sprintf("tag-%d", i)})
		}
		response := map[string]any{"items": items}
		if isTotalCountSent {
			response["totalCount"] = totalEntries
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(srv.Close)
	return srv, &requestCount
}

func TestListEntries_LoadsEveryPageBeyondTheFirstThousand(t *testing.T) {
	srv, _ := newPagedCatalogServer(t, 3760, true)

	entries, err := NewClient(srv.URL, "").ListEntries(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	if len(entries) != 3760 {
		t.Fatalf("got %d entries, want 3760", len(entries))
	}
	if entries[3759].ID != "entry-3759" {
		t.Errorf("last entry = %q, want entry-3759", entries[3759].ID)
	}
}

func TestListEntries_StopsAfterFirstPageWhenTotalCountIsMissing(t *testing.T) {
	srv, requestCount := newPagedCatalogServer(t, 3760, false)

	entries, err := NewClient(srv.URL, "").ListEntries(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	if requestCount.Load() != 1 {
		t.Errorf("sent %d requests, want 1", requestCount.Load())
	}
	if len(entries) != entriesPageSize {
		t.Errorf("got %d entries, want %d", len(entries), entriesPageSize)
	}
}

func TestListEntries_StopsOnEmptyPageEvenIfTotalCountIsHigher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"totalCount":5000}`))
	}))
	defer srv.Close()

	entries, err := NewClient(srv.URL, "").ListEntries(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestListEntries_StopsWhenServerIgnoresOffset(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"entry-0"},{"id":"entry-1"}],"totalCount":2147483647}`))
	}))
	defer srv.Close()

	entries, err := NewClient(srv.URL, "").ListEntries(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	if requestCount.Load() != 2 {
		t.Errorf("sent %d requests, want 2", requestCount.Load())
	}
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2 (the repeated page must not be appended)", len(entries))
	}
}

func TestListEntries_ReturnsErrorPastThePageCapWhenTotalCountIsBogus(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"items":[{"id":"entry-%s"}],"totalCount":2147483647}`, r.URL.Query().Get("offset"))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, "").ListEntries(context.Background(), "", ""); err == nil {
		t.Fatal("ListEntries returned nil error, want the page cap error")
	}
	if requestCount.Load() != maxEntriesPages {
		t.Errorf("sent %d requests, want %d", requestCount.Load(), maxEntriesPages)
	}
}

func TestListEntries_ReturnsErrorWhenALaterPageFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") != "0" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"entry-0"}],"totalCount":2000}`))
	}))
	defer srv.Close()

	if _, err := NewClient(srv.URL, "").ListEntries(context.Background(), "", ""); err == nil {
		t.Fatal("ListEntries returned nil error, want the page error")
	}
}

// newFilteringCatalogServer records each request's URL and serves /labels plus
// a one-entry page for every other path. The returned func snapshots the URLs.
func newFilteringCatalogServer(t *testing.T, labelsJSON string) (*httptest.Server, func() []*url.URL) {
	t.Helper()
	var mutex sync.Mutex
	var requests []*url.URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		requests = append(requests, r.URL)
		mutex.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/labels" {
			_, _ = w.Write([]byte(labelsJSON))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"entry-0"}],"totalCount":1}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []*url.URL {
		mutex.Lock()
		defer mutex.Unlock()
		return append([]*url.URL(nil), requests...)
	}
}

func TestListEntries_UsesSearchEndpointWithQueryWhenSearchIsSet(t *testing.T) {
	srv, recordedRequests := newFilteringCatalogServer(t, `{"items":[]}`)

	entries, err := NewClient(srv.URL, "").ListEntries(context.Background(), "", "temp")
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	requests := recordedRequests()
	last := requests[len(requests)-1]
	if last.Path != "/catalog-entries/search" {
		t.Errorf("path = %q, want /catalog-entries/search", last.Path)
	}
	if got := last.Query().Get("q"); got != "temp" {
		t.Errorf("q = %q, want temp", got)
	}
	if got := last.Query().Get("sourceTypes"); got != "DataBridge" {
		t.Errorf("sourceTypes = %q, want DataBridge", got)
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries, want 1", len(entries))
	}
}

func TestListEntries_ResolvesLabelNameToLabelIdForSearch(t *testing.T) {
	srv, recordedRequests := newFilteringCatalogServer(t, `{"items":[{"id":"label-analog","name":"Analog"}]}`)

	if _, err := NewClient(srv.URL, "").ListEntries(context.Background(), "analog", ""); err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	requests := recordedRequests()
	last := requests[len(requests)-1]
	if last.Path != "/catalog-entries/search" {
		t.Errorf("path = %q, want /catalog-entries/search", last.Path)
	}
	if got := last.Query().Get("labelIds"); got != "label-analog" {
		t.Errorf("labelIds = %q, want label-analog", got)
	}
	if !last.Query().Has("q") {
		t.Error("q is missing; /catalog-entries/search rejects a request without it")
	}
}

func TestListEntries_AcceptsLabelIdForSearch(t *testing.T) {
	srv, recordedRequests := newFilteringCatalogServer(t, `{"items":[{"id":"label-analog","name":"Analog"}]}`)

	if _, err := NewClient(srv.URL, "").ListEntries(context.Background(), "label-analog", ""); err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	requests := recordedRequests()
	if got := requests[len(requests)-1].Query().Get("labelIds"); got != "label-analog" {
		t.Errorf("labelIds = %q, want label-analog", got)
	}
}

func TestListEntries_ReturnsNoEntriesForUnknownLabel(t *testing.T) {
	srv, recordedRequests := newFilteringCatalogServer(t, `{"items":[{"id":"label-analog","name":"Analog"}]}`)

	entries, err := NewClient(srv.URL, "").ListEntries(context.Background(), "digital", "")
	if err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
	for _, request := range recordedRequests() {
		if request.Path != "/labels" {
			t.Errorf("unexpected request to %s; an unknown label must not list entries", request.Path)
		}
	}
}

func TestListEntries_KeepsListEndpointWithoutFilters(t *testing.T) {
	srv, recordedRequests := newFilteringCatalogServer(t, `{"items":[]}`)

	if _, err := NewClient(srv.URL, "").ListEntries(context.Background(), "", ""); err != nil {
		t.Fatalf("ListEntries returned error: %v", err)
	}

	requests := recordedRequests()
	if len(requests) != 1 || requests[0].Path != "/catalog-entries" {
		t.Errorf("requests = %v, want a single /catalog-entries call", requests)
	}
}
