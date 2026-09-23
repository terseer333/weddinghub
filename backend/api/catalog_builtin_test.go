package api

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// pages/dashboard.html and pages/event.html load templates/templates-data.js before
// template-engine.js, which seeds window.WEDDINGHUB_BUILTIN_TEMPLATES. That built-in list
// is what the studio renders from before /api/templates answers, and the only catalog when
// the API is unreachable. If it drifts from templates/templates.json, offline users see
// retired designs in the picker and never see newly added ones.
func TestBuiltinCatalogMatchesTemplateJSON(t *testing.T) {
	raw, err := os.ReadFile("../../templates/templates-data.js")
	if err != nil {
		t.Skipf("built-in catalog not available: %v", err)
	}

	const prefix = "window.WEDDINGHUB_BUILTIN_TEMPLATES = "
	src := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(src, prefix) {
		t.Fatalf("templates-data.js does not start with %q", prefix)
	}
	body := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(src, prefix)), ";")

	var builtin []map[string]any
	if err := json.Unmarshal([]byte(body), &builtin); err != nil {
		t.Fatalf("templates-data.js payload is not valid JSON: %v", err)
	}

	fileRaw, err := os.ReadFile("../../templates/templates.json")
	if err != nil {
		t.Skipf("catalog not available: %v", err)
	}
	var catalog []map[string]any
	if err := json.Unmarshal(fileRaw, &catalog); err != nil {
		t.Fatalf("templates.json is not valid JSON: %v", err)
	}

	if len(builtin) != len(catalog) {
		t.Fatalf("built-in catalog has %d entries, templates.json has %d", len(builtin), len(catalog))
	}

	// Compare entry by entry so a mismatch names the offending template.
	diverged := 0
	for i := range catalog {
		if !reflect.DeepEqual(builtin[i], catalog[i]) {
			id, _ := builtin[i]["id"].(string)
			if id == "" {
				id = "<missing>"
			}
			t.Errorf("entry %d differs (%s); regenerate with templates/sync-builtin-catalog.py", i, id)
			diverged++
		}
	}
	if diverged > 0 {
		t.Fatalf("%d of %d entries out of sync", diverged, len(catalog))
	}
}
