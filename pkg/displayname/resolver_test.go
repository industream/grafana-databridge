package displayname

import (
	"encoding/json"
	"testing"

	"github.com/industream/industream-data-bridge/pkg/datacatalog"
)

func makeEntry(name, tagLevel1 string, descriptions map[string]string) *datacatalog.CatalogEntry {
	var descRaw json.RawMessage
	if descriptions != nil {
		descRaw, _ = json.Marshal(descriptions)
	}
	return &datacatalog.CatalogEntry{
		ID:   "test-id",
		Name: name,
		Metadata: &datacatalog.CatalogMetadata{
			TagLevel1:      tagLevel1,
			DescriptionRaw: descRaw,
			Unit:           "degC",
		},
		Labels: []datacatalog.Label{{ID: "test-label", Name: "analog"}},
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

func TestResolve_DescriptionUsesTheViewerLanguage(t *testing.T) {
	entry := makeEntry("Parts counter", "", map[string]string{"en-US": "Parts counter", "de-DE": "Stückzähler", "fr-FR": "Compteur de pièces"})

	for locale, want := range map[string]string{"de-DE": "Stückzähler", "fr-FR": "Compteur de pièces", "en-US": "Parts counter"} {
		if got := Resolve("description", "", &ResolveContext{Entry: entry, Locale: locale}); got != want {
			t.Errorf("locale %s: expected %q, got %q", locale, want, got)
		}
	}
}

func TestResolve_DescriptionMatchesTheLanguageWhenTheRegionDiffers(t *testing.T) {
	entry := makeEntry("Parts counter", "", map[string]string{"en-US": "Parts counter", "de-DE": "Stückzähler"})

	if got := Resolve("description", "", &ResolveContext{Entry: entry, Locale: "de-AT"}); got != "Stückzähler" {
		t.Errorf("expected the de-DE text for a de-AT viewer, got %q", got)
	}
}

func TestResolve_DescriptionFallsBackToEnglishWithoutATranslation(t *testing.T) {
	entry := makeEntry("Parts counter", "", map[string]string{"en-US": "Parts counter EN", "de-DE": "Stückzähler"})

	if got := Resolve("description", "", &ResolveContext{Entry: entry, Locale: "pt-BR"}); got != "Parts counter EN" {
		t.Errorf("expected the English text, got %q", got)
	}
	if got := Resolve("description", "", &ResolveContext{Entry: entry}); got != "Parts counter EN" {
		t.Errorf("expected the English text when the locale is unknown, got %q", got)
	}
}

func TestResolve_DescriptionFallsBackToTheEntryName(t *testing.T) {
	entry := makeEntry("Parts counter", "", nil)

	if got := Resolve("description", "", &ResolveContext{Entry: entry, Locale: "fr-FR"}); got != "Parts counter" {
		t.Errorf("expected the entry name, got %q", got)
	}
}

func TestResolve_CustomPatternDescriptionPlaceholderFollowsTheViewerLanguage(t *testing.T) {
	entry := makeEntry("Parts counter", "", map[string]string{"en-US": "Parts counter", "fr-FR": "Compteur de pièces"})

	got := Resolve("custom", "{description} ({unit})", &ResolveContext{Entry: entry, Locale: "fr-FR"})
	if got != "Compteur de pièces (degC)" {
		t.Errorf("expected the French text in the pattern, got %q", got)
	}
}
