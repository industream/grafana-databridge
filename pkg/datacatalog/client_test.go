package datacatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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
