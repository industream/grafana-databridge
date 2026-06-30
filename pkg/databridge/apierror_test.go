package databridge

import (
	"strings"
	"testing"
)

func TestNewAPIError_ColumnDoesNotExist_FlagsNoData(t *testing.T) {
	body := []byte(`{"type":"https://tools.ietf.org/html/rfc4918#section-11.2","title":"Unable to query records.","status":422,"detail":"Records cannot be queried due to inconsistent content or semantic errors.","instance":"/records/query","code":"QueryRecords.Unprocessable","errors":[{"detail":"The column \"B040_JB020.MatMengeIst_0_Mat1\" does not exist.","code":"QueryRecords.ColumnDoesNotExist"}]}`)
	e := newAPIError(422, body)
	if !e.NoData {
		t.Fatalf("expected NoData=true for ColumnDoesNotExist")
	}
	if !strings.Contains(e.Detail, "does not exist") {
		t.Fatalf("expected Detail to carry the column message, got %q", e.Detail)
	}
	if e.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", e.StatusCode)
	}
}

func TestNewAPIError_OtherError_NotNoData(t *testing.T) {
	e := newAPIError(500, []byte(`{"detail":"boom"}`))
	if e.NoData {
		t.Fatalf("expected NoData=false for non-column error")
	}
	if e.Detail != "boom" {
		t.Fatalf("expected fallback to top-level detail, got %q", e.Detail)
	}
}
