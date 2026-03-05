package databridge

import (
	"testing"
)

func TestToDataFrame_EmptyResponse(t *testing.T) {
	frame, err := ToDataFrame("test", &RecordsResponse{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if frame.Name != "test" {
		t.Errorf("expected name 'test', got %q", frame.Name)
	}
	if len(frame.Fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(frame.Fields))
	}
}

func TestToDataFrame_NilResponse(t *testing.T) {
	frame, err := ToDataFrame("test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(frame.Fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(frame.Fields))
	}
}

func TestToDataFrame_FloatColumn(t *testing.T) {
	resp := &RecordsResponse{
		Columns: []RecordsColumn{
			{Name: "temperature", DataType: "float64"},
		},
		Items: [][]interface{}{
			{25.5},
			{30.1},
			{nil},
		},
	}

	frame, err := ToDataFrame("test", resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(frame.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(frame.Fields))
	}
	if frame.Fields[0].Name != "temperature" {
		t.Errorf("expected field name 'temperature', got %q", frame.Fields[0].Name)
	}
	if frame.Fields[0].Len() != 3 {
		t.Errorf("expected 3 rows, got %d", frame.Fields[0].Len())
	}

	// Check non-nil value
	val := frame.Fields[0].At(0).(*float64)
	if val == nil || *val != 25.5 {
		t.Errorf("expected 25.5, got %v", val)
	}

	// Check nil value
	nilVal := frame.Fields[0].At(2)
	if nilVal != (*float64)(nil) {
		t.Errorf("expected nil, got %v", nilVal)
	}
}

func TestToDataFrame_TimeColumn(t *testing.T) {
	resp := &RecordsResponse{
		Columns: []RecordsColumn{
			{Name: "time", DataType: "dateTime"},
		},
		Items: [][]interface{}{
			{"2025-01-01T00:00:00Z"},
			{"2025-01-01T01:00:00Z"},
		},
	}

	frame, err := ToDataFrame("test", resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if frame.Fields[0].Len() != 2 {
		t.Fatalf("expected 2 rows, got %d", frame.Fields[0].Len())
	}
}

func TestToDataFrame_BoolColumn(t *testing.T) {
	resp := &RecordsResponse{
		Columns: []RecordsColumn{
			{Name: "status", DataType: "bool"},
		},
		Items: [][]interface{}{
			{true},
			{false},
		},
	}

	frame, err := ToDataFrame("test", resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val := frame.Fields[0].At(0).(*bool)
	if val == nil || *val != true {
		t.Errorf("expected true, got %v", val)
	}
}

func TestToDataFrame_MultipleColumns(t *testing.T) {
	resp := &RecordsResponse{
		Columns: []RecordsColumn{
			{Name: "time", DataType: "dateTime"},
			{Name: "value", DataType: "float64"},
			{Name: "label", DataType: "string"},
		},
		Items: [][]interface{}{
			{"2025-01-01T00:00:00Z", 25.5, "sensor-a"},
		},
	}

	frame, err := ToDataFrame("test", resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(frame.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(frame.Fields))
	}
}
