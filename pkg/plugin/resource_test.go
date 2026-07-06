package plugin

import (
	"testing"

	"github.com/industream/industream-data-bridge/pkg/datacatalog"
)

// Regression: DataCatalog's binding model gives each entry a distinct `id`
// (binding id) and `entryId` (logical id). Asset nodes reference the logical
// `entryId`, so the plugin must match node entryIds against CatalogEntry.EntryID,
// not CatalogEntry.ID — otherwise every tag under a node is filtered out.
func TestBuildValidEntryIds_KeysOnLogicalEntryId(t *testing.T) {
	entries := []datacatalog.CatalogEntry{
		{ID: "binding-1", EntryID: "logical-1"},
		{ID: "binding-2", EntryID: "logical-2"},
	}
	valid := buildValidEntryIds(entries, "")
	if !valid["logical-1"] || !valid["logical-2"] {
		t.Fatalf("expected logical ids in valid set, got %v", valid)
	}
	if valid["binding-1"] || valid["binding-2"] {
		t.Errorf("binding ids must not be the match key (nodes reference logical ids)")
	}
}

func TestBuildValidEntryIds_FallsBackToIdWhenNoEntryId(t *testing.T) {
	entries := []datacatalog.CatalogEntry{{ID: "solo-1"}}
	valid := buildValidEntryIds(entries, "")
	if !valid["solo-1"] {
		t.Fatalf("expected fallback to id when entryId is empty, got %v", valid)
	}
}

func TestFilterTreeEntryIds_KeepsNodeEntriesMatchingLogicalIds(t *testing.T) {
	entries := []datacatalog.CatalogEntry{{ID: "binding-1", EntryID: "logical-1"}}
	valid := buildValidEntryIds(entries, "")
	nodes := []datacatalog.AssetNode{{ID: "n1", EntryIds: []string{"logical-1", "logical-missing"}}}
	filterTreeEntryIds(nodes, valid)
	if nodes[0].EntryCount != 1 || len(nodes[0].EntryIds) != 1 || nodes[0].EntryIds[0] != "logical-1" {
		t.Fatalf("expected node to keep logical-1, got count=%d ids=%v", nodes[0].EntryCount, nodes[0].EntryIds)
	}
}

func TestSelectEntriesByLogicalIds_ResolvesNodeIdsToBindingEntries(t *testing.T) {
	entries := []datacatalog.CatalogEntry{
		{ID: "binding-1", EntryID: "logical-1", Name: "A"},
		{ID: "binding-2", EntryID: "logical-2", Name: "B"},
	}
	got := selectEntriesByLogicalIds(entries, []string{"logical-2"})
	if len(got) != 1 || got[0].ID != "binding-2" {
		t.Fatalf("expected binding-2 for logical-2, got %v", got)
	}
}

func TestSplitIds(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"abc", []string{"abc"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitIds(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("item %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}
