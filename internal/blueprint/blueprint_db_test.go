package blueprint_test

import (
	"context"
	"testing"
	"time"

	"kanban/internal/blueprint"
	"kanban/internal/db/dbtest"
	"kanban/internal/store"
)

func TestEveryBlueprintBuilds(t *testing.T) {
	s := store.New(dbtest.New(t))
	ctx := context.Background()
	lead, err := s.UpsertIdentity(ctx, store.Identity{Issuer: "https://idp.test", Subject: "lead", Email: "lead@example.com", Name: "Lead"}, nil, "tr")
	if err != nil {
		t.Fatal(err)
	}
	team, err := s.CreateTeam(ctx, "Platform")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, team.ID, lead.ID, store.RoleLead); err != nil {
		t.Fatal(err)
	}
	for _, locale := range []string{"tr", "en"} {
		tr := strict(t, locale)
		for _, bp := range blueprint.All() {
			before := time.Now().Add(-time.Second)
			plan := blueprint.Plan(bp, tr, blueprint.AllOptions())
			b, err := s.CreateBoardFromPlan(ctx, team.ID, locale+" "+bp.Key, plan, lead.ID)
			if err != nil {
				t.Fatalf("%s/%s: %v", locale, bp.Key, err)
			}
			cols, _ := s.Columns(ctx, b.ID)
			labels, _ := s.Labels(ctx, b.ID)
			tpls, _ := s.Templates(ctx, b.ID)
			rules, _ := s.BoardRules(ctx, b.ID)
			c := bp.Counts()
			if len(cols) != c.Columns || len(labels) != c.Labels || len(tpls) != c.Templates ||
				len(rules.Permissions)+len(rules.Conditions) > c.Rules {
				t.Errorf("%s/%s: built %d cols %d labels %d templates %+v, want %+v", locale, bp.Key, len(cols), len(labels), len(tpls), rules, c)
			}
			for _, tp := range tpls {
				if tp.Schedule.Kind != "" && (tp.ScheduleSince == nil || tp.ScheduleSince.Before(before)) {
					t.Errorf("%s/%s: %q schedule starts %v, before the board", locale, bp.Key, tp.Name, tp.ScheduleSince)
				}
				if _, err := s.CreateCardFromTemplate(ctx, b.ID, tp.ID, 0, "", lead.ID, time.UTC, time.Now()); err != nil {
					t.Errorf("%s/%s: a card from %q: %v", locale, bp.Key, tp.Name, err)
				}
			}
		}
	}
}
