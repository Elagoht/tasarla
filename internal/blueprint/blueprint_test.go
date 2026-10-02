package blueprint_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"kanban/internal/blueprint"
	"kanban/internal/store"
)

// catalog reads a locale's catalog flat, "a.b.c" → text; plural objects
// (one/other) are left out, as blueprints use none.
func catalog(t *testing.T, locale string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("../../locales/" + locale + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]json.RawMessage
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	var walk func(prefix string, m map[string]json.RawMessage)
	walk = func(prefix string, m map[string]json.RawMessage) {
		for k, v := range m {
			var s string
			if json.Unmarshal(v, &s) == nil {
				out[prefix+k] = s
				continue
			}
			var sub map[string]json.RawMessage
			if json.Unmarshal(v, &sub) == nil {
				walk(prefix+k+".", sub)
			}
		}
	}
	walk("", tree)
	return out
}

// strict translates as the Strict catalogs do, and fails on a key that is missing.
func strict(t *testing.T, locale string) func(string) string {
	cat := catalog(t, locale)
	return func(key string) string {
		s, ok := cat[key]
		if !ok || s == "" {
			t.Errorf("%s: no text for %q", locale, key)
		}
		return s
	}
}

var palette = []string{"#e03131", "#f08c00", "#2f9e44", "#1971c2", "#7048e8", "#c2255c", "#0c8599", "#495057"}

func TestCatalogOrderAndDefault(t *testing.T) {
	var keys []string
	for _, bp := range blueprint.All() {
		keys = append(keys, bp.Key)
	}
	want := []string{"simple", "scrum", "bugs", "software", "content", "hiring", "support"}
	if !slices.Equal(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if _, ok := blueprint.Find(blueprint.Default); !ok || blueprint.Default != "simple" {
		t.Fatal("the default is not simple")
	}
	if _, ok := blueprint.Find("nope"); ok {
		t.Fatal("an unknown key was found")
	}
}

// Every blueprint renders in every locale with no text missing, and every
// index it holds points at something that exists.
func TestEveryBlueprintRendersInEveryLocale(t *testing.T) {
	for _, locale := range []string{"tr", "en"} {
		tr := strict(t, locale)
		for _, bp := range blueprint.All() {
			tr(bp.NameKey())
			tr(bp.SummaryKey())
			p := blueprint.Render(bp, tr)
			if len(p.Columns) < 3 {
				t.Errorf("%s: %d columns", bp.Key, len(p.Columns))
			}
			done := 0
			for i, c := range p.Columns {
				if c.Name == "" {
					t.Errorf("%s/%s: column %d has no name", locale, bp.Key, i)
				}
				if c.Done {
					done++
					if i != len(p.Columns)-1 {
						t.Errorf("%s: done column %d is not last", bp.Key, i)
					}
				}
				if c.AllowCreate != (i == 0) {
					t.Errorf("%s: column %d AllowCreate = %v", bp.Key, i, c.AllowCreate)
				}
			}
			if done != 1 {
				t.Errorf("%s: %d done columns", bp.Key, done)
			}
			for _, l := range p.Labels {
				if l.Name == "" || !slices.Contains(palette, l.Color) {
					t.Errorf("%s: label %+v", bp.Key, l)
				}
			}
			for _, tp := range p.Templates {
				if tp.Name == "" || tp.Title == "" || tp.Description == "" || len(tp.Checklist) == 0 || slices.Contains(tp.Checklist, "") {
					t.Errorf("%s/%s: template %+v", locale, bp.Key, tp)
				}
				if tp.ColumnIndex != 0 {
					t.Errorf("%s: template %q targets column %d", bp.Key, tp.Name, tp.ColumnIndex)
				}
				for _, li := range tp.LabelIndexes {
					if li < 0 || li >= len(p.Labels) {
						t.Errorf("%s: template label index %d", bp.Key, li)
					}
				}
			}
			for _, pm := range p.Permissions {
				if pm.ToIndex < 0 || pm.ToIndex >= len(p.Columns) || !slices.Contains([]string{"team_lead", "assignee", "any_member"}, pm.Subject) {
					t.Errorf("%s: permission %+v", bp.Key, pm)
				}
			}
			for _, c := range p.Conditions {
				if c.ColumnIndex <= 0 || c.ColumnIndex >= len(p.Columns) {
					t.Errorf("%s: condition on column %d (the first column takes new cards and keeps no gate)", bp.Key, c.ColumnIndex)
				}
			}
		}
	}
}

func TestHasAndCounts(t *testing.T) {
	simple, _ := blueprint.Find("simple")
	if simple.Has() != (blueprint.Options{}) || simple.Counts() != (blueprint.Counts{Columns: 3}) {
		t.Fatalf("simple: %+v %+v", simple.Has(), simple.Counts())
	}
	scrum, _ := blueprint.Find("scrum")
	if scrum.Has() != blueprint.AllOptions() {
		t.Fatalf("scrum has %+v", scrum.Has())
	}
	if c := scrum.Counts(); c != (blueprint.Counts{Columns: 5, Labels: 4, Templates: 3, Rules: 2}) {
		t.Fatalf("scrum counts %+v", c)
	}
	bugs, _ := blueprint.Find("bugs")
	if h := bugs.Has(); h.Recurring || !h.Templates || !h.Rules || !h.WIP || !h.Labels {
		t.Fatalf("bugs has %+v", h)
	}
	software, _ := blueprint.Find("software")
	if c := software.Counts(); c.Rules != 2 { // one condition and the person WIP limit
		t.Fatalf("software rules = %d", c.Rules)
	}
}

func TestTrim(t *testing.T) {
	scrum, _ := blueprint.Find("scrum")
	full := blueprint.Render(scrum, strict(t, "en"))

	none := blueprint.Trim(full, blueprint.Options{})
	if len(none.Columns) != 5 || len(none.Labels) != 0 || len(none.Templates) != 0 || len(none.Permissions) != 0 ||
		len(none.Conditions) != 0 || none.PersonWIP != nil {
		t.Fatalf("nothing kept %+v", none)
	}
	for _, c := range none.Columns {
		if c.WIP != nil {
			t.Fatalf("WIP kept on %q", c.Name)
		}
	}

	noLabels := blueprint.Trim(full, blueprint.Options{Templates: true, Recurring: true})
	for _, tp := range noLabels.Templates {
		if len(tp.LabelIndexes) != 0 {
			t.Fatalf("template %q keeps labels without labels", tp.Name)
		}
	}

	noRecurring := blueprint.Trim(full, blueprint.Options{Templates: true})
	for _, tp := range noRecurring.Templates {
		if tp.Schedule.Kind != "" {
			t.Fatalf("template %q keeps its schedule", tp.Name)
		}
	}

	if !slices.EqualFunc(blueprint.Trim(full, blueprint.AllOptions()).Columns, full.Columns, func(a, b store.PlanColumn) bool {
		return a.Name == b.Name && (a.WIP == nil) == (b.WIP == nil)
	}) {
		t.Fatal("all options changed the columns")
	}
}

// "Recurring" without "templates" builds no template, so no schedule.
func TestTrimRecurringNeedsTemplates(t *testing.T) {
	scrum, _ := blueprint.Find("scrum")
	p := blueprint.Plan(scrum, strict(t, "tr"), blueprint.Options{Recurring: true})
	if len(p.Templates) != 0 {
		t.Fatalf("templates = %d", len(p.Templates))
	}
}

func TestOptionsFrom(t *testing.T) {
	got := blueprint.OptionsFrom([]string{"labels", "rules", "rules", "bogus"})
	if got != (blueprint.Options{Labels: true, Rules: true}) {
		t.Fatalf("got %+v", got)
	}
}
