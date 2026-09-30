package store_test

import (
	"context"
	"errors"
	"testing"

	"kanban/internal/store"
)

func TestTeamsAndMembers(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	admin := mustUpsert(t, s, identity("admin", "admin@example.com"), "admin@example.com")
	lead := mustUpsert(t, s, identity("lead", "lead@example.com"))
	member := mustUpsert(t, s, identity("member", "member@example.com"))
	outsider := mustUpsert(t, s, identity("out", "out@example.com"))

	team, err := s.CreateTeam(ctx, "Platform")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateTeam(ctx, "Another")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, team.ID, lead.ID, store.RoleLead); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, team.ID, member.ID, store.RoleMember); err != nil {
		t.Fatal(err)
	}

	all, err := s.ListTeams(ctx, admin)
	if err != nil || len(all) != 2 {
		t.Fatalf("admin ListTeams = %d teams, %v; want 2", len(all), err)
	}
	mine, err := s.ListTeams(ctx, member)
	if err != nil || len(mine) != 1 || mine[0].Team.ID != team.ID || mine[0].Role != store.RoleMember || mine[0].MemberCount != 2 {
		t.Fatalf("member ListTeams = %+v, %v", mine, err)
	}
	if none, _ := s.ListTeams(ctx, outsider); len(none) != 0 {
		t.Fatalf("outsider sees %d teams", len(none))
	}
	if adminOwn, _ := s.TeamsOf(ctx, admin.ID); len(adminOwn) != 0 {
		t.Fatalf("TeamsOf(admin) = %d, want only teams admin belongs to (0)", len(adminOwn))
	}
	_ = other

	members, err := s.Members(ctx, team.ID)
	if err != nil || len(members) != 2 || members[0].User.ID != lead.ID {
		t.Fatalf("Members = %+v, %v; want lead first", members, err)
	}

	if role, err := s.MemberRole(ctx, team.ID, lead.ID); err != nil || role != store.RoleLead {
		t.Errorf("MemberRole(lead) = %q, %v", role, err)
	}
	if _, err := s.MemberRole(ctx, team.ID, outsider.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("MemberRole(outsider): %v", err)
	}

	if err := s.SetRole(ctx, team.ID, member.ID, store.RoleLead); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, team.ID, member.ID, store.RoleMember); err != nil {
		t.Fatalf("re-adding an existing member: %v", err)
	}
	if role, _ := s.MemberRole(ctx, team.ID, member.ID); role != store.RoleMember {
		t.Errorf("AddMember on an existing member did not update the role: %q", role)
	}
	if err := s.SetRole(ctx, team.ID, outsider.ID, store.RoleLead); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetRole(outsider): %v", err)
	}

	if err := s.RemoveMember(ctx, team.ID, member.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveMember(ctx, team.ID, member.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("removing twice: %v", err)
	}
}

func TestTeamLookups(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if _, err := s.Team(ctx, 12345); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Team(unknown): %v", err)
	}
	if _, err := s.CreateTeam(ctx, "   "); err == nil {
		t.Error("a blank team name was stored")
	}
	if _, ok := store.ParseRole("owner"); ok {
		t.Error("ParseRole accepted an unknown role")
	}
	if r, ok := store.ParseRole("lead"); !ok || r != store.RoleLead {
		t.Error("ParseRole(lead)")
	}
}
