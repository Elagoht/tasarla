package web

import (
	"context"
	"slices"
	"strconv"

	"kanban/internal/notify"
	"kanban/internal/store"
)

// The events below are emitted after the change they describe has committed.
// A failure to notify is logged by the notifier and never fails the action.

func (h *handlers) notifyAssigned(ctx context.Context, bc boardContext, before, after store.Card) {
	if after.AssigneeID == nil || (before.AssigneeID != nil && *before.AssigneeID == *after.AssigneeID) {
		return
	}
	h.notifier.Emit(ctx, notify.Event{
		Kind: store.NotifyAssigned, To: *after.AssigneeID, Actor: bc.User.ID, ActorName: bc.User.Name,
		Card: after, BoardName: bc.Board.Name,
	})
}

func excerpt(body string) string {
	r := []rune(body)
	if len(r) > 200 {
		r = append(r[:200], '…')
	}
	return string(r)
}

func (h *handlers) mentionEvents(bc boardContext, card store.Card, body string, mentions []int64) []notify.Event {
	var events []notify.Event
	for _, id := range mentions {
		events = append(events, notify.Event{Kind: store.NotifyMentioned, To: id, Actor: bc.User.ID, ActorName: bc.User.Name,
			Card: card, BoardName: bc.Board.Name, Text: excerpt(body)})
	}
	return events
}

// notifyMentioned tells the people a comment newly mentions.
func (h *handlers) notifyMentioned(ctx context.Context, bc boardContext, card store.Card, body string, mentions []int64) {
	h.notifier.Emit(ctx, h.mentionEvents(bc, card, body, mentions)...)
}

func (h *handlers) notifyComment(ctx context.Context, bc boardContext, card store.Card, body string, mentions []int64) {
	events := h.mentionEvents(bc, card, body, mentions)
	if card.AssigneeID != nil && !slices.Contains(mentions, *card.AssigneeID) {
		events = append(events, notify.Event{Kind: store.NotifyCommented, To: *card.AssigneeID, Actor: bc.User.ID, ActorName: bc.User.Name,
			Card: card, BoardName: bc.Board.Name, Text: excerpt(body)})
	}
	h.notifier.Emit(ctx, events...)
}

// notifyUnblocked tells the assignees of the cards finished no longer wait on.
func (h *handlers) notifyUnblocked(ctx context.Context, bc boardContext, finished int64) {
	cards, err := h.store.NewlyUnblocked(ctx, finished)
	if err != nil {
		h.log.Error("notify: unblocked", "card", finished, "err", err)
		return
	}
	var events []notify.Event
	for _, c := range cards {
		if c.AssigneeID == nil {
			continue
		}
		deps, err := h.store.CardDependencies(ctx, c.ID)
		if err != nil {
			h.log.Error("notify: unblocked", "card", c.ID, "err", err)
			continue
		}
		// Once per person, card and set of blockers: finishing the same
		// blocker again, or archiving it after, says nothing new.
		key := "unblocked:" + strconv.FormatInt(*c.AssigneeID, 10) + ":" + strconv.FormatInt(c.ID, 10)
		for _, b := range deps.Blockers {
			key += ":" + strconv.FormatInt(b.ID, 10)
		}
		events = append(events, notify.Event{Kind: store.NotifyUnblocked, To: *c.AssigneeID, Actor: bc.User.ID,
			ActorName: bc.User.Name, Card: c, BoardName: bc.Board.Name, DedupeKey: key})
	}
	h.notifier.Emit(ctx, events...)
}

// finishedBy reports whether a card now sits in a done column.
func (h *handlers) finishedBy(ctx context.Context, card store.Card) bool {
	cols, err := h.store.Columns(ctx, card.BoardID)
	if err != nil {
		return false
	}
	for _, c := range cols {
		if c.ID == card.ColumnID {
			return c.IsDone
		}
	}
	return false
}
