package notify

import (
	"context"
	"strconv"
	"time"

	"kanban/internal/schedule"
	"kanban/internal/store"
)

// Recurrer makes the cards that card templates' schedules call for.
type Recurrer struct {
	Store    *store.Store
	Notifier *Notifier
	Location *time.Location // nil: UTC
	Interval time.Duration  // default 5m
	Now      func() time.Time
}

// Tick runs each scheduled template's latest moment, once. Moments missed
// while the app was down are not made up for: only the latest is run. A card
// made is shown on open boards, and its assignee hears of it from the
// template's last editor. A moment that made no card is told to the template's last editor, or to the
// team's leads when that editor has left the team. A template that fails is
// logged and the others go on.
func (r Recurrer) Tick(ctx context.Context) error {
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	loc := r.Location
	if loc == nil {
		loc = time.UTC
	}
	templates, err := r.Store.ScheduledTemplates(ctx)
	if err != nil {
		return err
	}
	for _, st := range templates {
		tpl := st.Template
		at := schedule.Latest(schedule.Spec(tpl.Schedule), loc, now())
		if at.IsZero() || at.Before(*tpl.ScheduleSince) {
			continue
		}
		out, err := r.Store.RunTemplate(ctx, st, at, loc)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.Notifier.Log.Error("notify: recurrer", "template", tpl.ID, "at", at, "err", err)
			continue
		}
		if out.Card != nil {
			r.made(ctx, st, out)
			continue
		}
		if !out.Ran {
			continue
		}
		recipients := []int64{out.Owner}
		if out.OwnerGone {
			if recipients, err = r.Store.TeamLeads(ctx, st.TeamID); err != nil {
				r.Notifier.Log.Error("notify: recurrer leads", "team", st.TeamID, "err", err)
				continue
			}
		}
		events := make([]Event, 0, len(recipients))
		for _, to := range recipients {
			events = append(events, Event{
				Kind: store.NotifyTemplateFailed, To: to,
				Card:      store.Card{BoardID: tpl.BoardID, Title: tpl.Name},
				BoardName: st.BoardName,
				DedupeKey: store.NotifyTemplateFailed + ":" + strconv.FormatInt(tpl.ID, 10) + ":" +
					at.Format(time.RFC3339) + ":" + strconv.FormatInt(to, 10),
			})
		}
		// Emit logs each failure itself.
		_ = r.Notifier.Emit(ctx, events...)
	}
	return nil
}

// made tells of the card a run made: open boards get it, and its assignee
// hears they were assigned, by the template's last editor.
func (r Recurrer) made(ctx context.Context, st store.ScheduledTemplate, out store.RunOutcome) {
	card := *out.Card
	if err := r.Notifier.Invalidate(ctx, "board:"+strconv.FormatInt(card.BoardID, 10)); err != nil {
		r.Notifier.Log.Error("notify: recurrer invalidate", "board", card.BoardID, "err", err)
	}
	if card.AssigneeID == nil {
		return
	}
	var actorName string
	if owner, err := r.Store.UserByID(ctx, out.Owner); err != nil {
		r.Notifier.Log.Error("notify: recurrer owner", "user", out.Owner, "err", err)
	} else {
		actorName = owner.Name
	}
	// Emit logs a failure itself.
	_ = r.Notifier.Emit(ctx, Event{
		Kind: store.NotifyAssigned, To: *card.AssigneeID, Actor: out.Owner, ActorName: actorName,
		Card: card, BoardName: st.BoardName,
	})
}

// Run ticks until ctx is cancelled.
func (r Recurrer) Run(ctx context.Context) {
	every(ctx, r.Interval, 5*time.Minute, func() {
		if err := r.Tick(ctx); err != nil && ctx.Err() == nil {
			r.Notifier.Log.Error("notify: recurrer", "err", err)
		}
	})
}
