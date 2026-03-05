package displayname

import (
	"testing"

	"github.com/industream/industream-data-bridge/pkg/datacatalog"
)

func makeEntry(name, tagLevel1 string, descriptions map[string]string) *datacatalog.CatalogEntry {
	return &datacatalog.CatalogEntry{
		ID:   "test-id",
		Name: name,
		Metadata: &datacatalog.CatalogMetadata{
			TagLevel1:   tagLevel1,
			Description: descriptions,
			Unit:        "degC",
		},
		Labels: []string{"analog"},
	}
}

func TestResolve_EntryName(t *testing.T) {
	entry := makeEntry("Temperature Basket", "DB01_T_BASKET", nil)
	ctx := &ResolveContext{Entry: entry}

	result := Resolve("entryName", "", ctx)
	if result != "Temperature Basket" {
		t.Errorf("expected 'Temperature Basket', got %q", result)
	}
}

func TestResolve_TagLevel1(t *testing.T) {
	entry := makeEntry("Temperature Basket", "DB01_T_BASKET", nil)
	ctx := &ResolveContext{Entry: entry}

	result := Resolve("tagLevel1", "", ctx)
	if result != "DB01_T_BASKET" {
		t.Errorf("expected 'DB01_T_BASKET', got %q", result)
	}
}

func TestResolve_TagLevel1_FallbackToName(t *testing.T) {
	entry := makeEntry("Temperature Basket", "", nil)
	ctx := &ResolveContext{Entry: entry}

	result := Resolve("tagLevel1", "", ctx)
	if result != "Temperature Basket" {
		t.Errorf("expected fallback to name, got %q", result)
	}
}

func TestResolve_DescriptionEn(t *testing.T) {
	entry := makeEntry("Temperature Basket", "", map[string]string{
		"en-US": "Basket temperature actual",
		"de-DE": "Temperatur Korb aktuell",
	})
	ctx := &ResolveContext{Entry: entry}

	result := Resolve("descriptionEn", "", ctx)
	if result != "Basket temperature actual" {
		t.Errorf("expected English description, got %q", result)
	}
}

func TestResolve_DescriptionDe(t *testing.T) {
	entry := makeEntry("Temperature Basket", "", map[string]string{
		"en-US": "Basket temperature actual",
		"de-DE": "Temperatur Korb aktuell",
	})
	ctx := &ResolveContext{Entry: entry}

	result := Resolve("descriptionDe", "", ctx)
	if result != "Temperatur Korb aktuell" {
		t.Errorf("expected German description, got %q", result)
	}
}

func TestResolve_AssetPath(t *testing.T) {
	entry := makeEntry("Temperature Basket", "", nil)
	ctx := &ResolveContext{Entry: entry, AssetPath: "EAF > Furnace PLC > Temperature Basket"}

	result := Resolve("assetPath", "", ctx)
	if result != "EAF > Furnace PLC > Temperature Basket" {
		t.Errorf("expected asset path, got %q", result)
	}
}

func TestResolve_CustomPattern(t *testing.T) {
	entry := makeEntry("Temperature Basket", "DB01_T_BASKET", map[string]string{
		"en-US": "Basket temp",
	})
	ctx := &ResolveContext{
		Entry:       entry,
		Column:      "temperature",
		Aggregation: "avg",
		AssetPath:   "EAF > Furnace",
		Connection:  "PostgreSQL",
	}

	result := Resolve("custom", "{name} [{tagLevel1}] ({aggregation})", ctx)
	expected := "Temperature Basket [DB01_T_BASKET] (avg)"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolve_NilContext(t *testing.T) {
	result := Resolve("entryName", "", nil)
	if result != "unknown" {
		t.Errorf("expected 'unknown', got %q", result)
	}
}

func TestResolve_NilEntry_WithColumn(t *testing.T) {
	ctx := &ResolveContext{Column: "temperature"}
	result := Resolve("entryName", "", ctx)
	if result != "temperature" {
		t.Errorf("expected column name fallback, got %q", result)
	}
}

func TestResolve_EmptyPreset(t *testing.T) {
	entry := makeEntry("Temperature Basket", "", nil)
	ctx := &ResolveContext{Entry: entry}

	result := Resolve("", "", ctx)
	if result != "Temperature Basket" {
		t.Errorf("expected entry name as default, got %q", result)
	}
}
