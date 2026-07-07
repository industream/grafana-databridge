package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestBuildStatsQuery_UsesColumnsAsEntries(t *testing.T) {
	tr := backend.TimeRange{
		From: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2025, 6, 2, 0, 0, 0, 0, time.UTC),
	}

	sq := buildStatsQuery([]string{"ramp", "const42"}, []string{"p50", "mean"}, tr)

	// A DataBridge stats "entry" is a signal = a column/_field (post fix #96).
	if len(sq.Entries) != 2 || sq.Entries[0] != "ramp" || sq.Entries[1] != "const42" {
		t.Fatalf("entries = %v, want [ramp const42]", sq.Entries)
	}
	if sq.Compute[0] != "p50" || sq.Compute[1] != "mean" {
		t.Fatalf("compute = %v", sq.Compute)
	}
	if sq.Start != "2025-06-01T00:00:00Z" || sq.End != "2025-06-02T00:00:00Z" {
		t.Fatalf("range = %s..%s", sq.Start, sq.End)
	}
}

func TestBuildStatsQuery_DefaultsComputeWhenEmpty(t *testing.T) {
	tr := backend.TimeRange{From: time.Now(), To: time.Now()}

	sq := buildStatsQuery([]string{"x"}, nil, tr)

	if len(sq.Compute) == 0 {
		t.Fatal("expected default compute stats when none selected")
	}
}

func TestStatsToFrame_OneRowPerSignal_NullForMissingStat(t *testing.T) {
	rows := []statsRow{
		{signal: "Ramp", values: map[string]float64{"p50": 49.5, "mean": 49.5}},
		{signal: "Const", values: map[string]float64{"p50": 42}}, // mean missing
	}

	frame := statsToFrame("A", []string{"p50", "mean"}, rows)

	// Fields: Signal + p50 + mean
	if len(frame.Fields) != 3 {
		t.Fatalf("fields = %d, want 3", len(frame.Fields))
	}
	if frame.Fields[0].Name != "Signal" || frame.Fields[1].Name != "p50" || frame.Fields[2].Name != "mean" {
		t.Fatalf("field names = %s/%s/%s", frame.Fields[0].Name, frame.Fields[1].Name, frame.Fields[2].Name)
	}
	if frame.Fields[0].Len() != 2 {
		t.Fatalf("rows = %d, want 2", frame.Fields[0].Len())
	}
	// Const.mean must be null (nil *float64).
	if meanConst, _ := frame.Fields[2].At(1).(*float64); meanConst != nil {
		t.Fatalf("Const.mean = %v, want nil", *meanConst)
	}
	// Ramp.p50 present.
	p50Ramp, ok := frame.Fields[1].At(0).(*float64)
	if !ok || p50Ramp == nil || *p50Ramp != 49.5 {
		t.Fatalf("Ramp.p50 = %v, want 49.5", frame.Fields[1].At(0))
	}
}
