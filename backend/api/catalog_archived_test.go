package api

import (
	"encoding/json"
	"os"
	"testing"

	"weddinghub/models"
)

// The catalog reaches the client from templates/templates.json. Archived entries have to
// survive that JSON round-trip: the client hides them from the picker but still needs them
// to resolve the design of weddings that already reference one. If the Archived field is
// ever dropped from models.Template, every retired template silently returns to the gallery.
func TestArchivedTemplatesSurviveCatalogRoundTrip(t *testing.T) {
	const path = "../../templates/templates.json"

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("catalog not available at %s: %v", path, err)
	}

	var catalog []models.Template
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatalf("catalog does not unmarshal into []models.Template: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("catalog is empty")
	}

	archived, active := 0, 0
	for _, tpl := range catalog {
		if tpl.Archived {
			archived++
		} else {
			active++
		}
		if tpl.ID == "" || tpl.Name == "" || tpl.Category == "" {
			t.Errorf("template %q is missing required identity fields", tpl.ID)
		}
	}

	if archived == 0 {
		t.Error("expected some archived templates; the archived flag is likely being dropped")
	}
	if active == 0 {
		t.Error("expected some active templates")
	}
	if active != 30 {
		t.Errorf("active template count = %d, want 30 (the curated gallery size)", active)
	}

	// Re-encoding, as listTemplates does, must preserve the flag for every archived entry.
	round, err := json.Marshal(catalog)
	if err != nil {
		t.Fatalf("failed to re-marshal catalog: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(round, &decoded); err != nil {
		t.Fatalf("failed to decode re-marshalled catalog: %v", err)
	}
	flagged := 0
	for _, item := range decoded {
		if v, ok := item["archived"]; ok && v == true {
			flagged++
		}
	}
	if flagged != archived {
		t.Errorf("archived flag survived encoding for %d of %d entries", flagged, archived)
	}
}
