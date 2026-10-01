package notify

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"kanban/internal/store"
)

// Worker sends the outbox (spec §10).
type Worker struct {
	Store    *store.Store
	Send     func(ctx context.Context, m store.OutboxMessage) error
	Interval time.Duration // default 30s
	Log      *slog.Logger
}

// Tick sends whatever is due now.
func (w Worker) Tick(ctx context.Context) error {
	sent, failed, err := w.Store.ProcessOutbox(ctx, time.Now(), 20, func(m store.OutboxMessage) error {
		// An e-mail being sent when shutdown begins is finished, not cut off.
		err := w.Send(context.WithoutCancel(ctx), m)
		if err != nil {
			w.Log.Warn("notify: send failed", "outbox", m.ID, "err", err)
		}
		return err
	}, func(m store.OutboxMessage, err error) {
		w.Log.Error("notify: e-mail given up", "outbox", m.ID, "to", m.To, "err", err)
	})
	if sent+failed > 0 {
		w.Log.Info("notify: outbox", "sent", sent, "failed", failed)
	}
	return err
}

// Run ticks until ctx is cancelled.
func (w Worker) Run(ctx context.Context) {
	every(ctx, w.Interval, 30*time.Second, func() {
		if err := w.Tick(ctx); err != nil && ctx.Err() == nil {
			w.Log.Error("notify: outbox", "err", err)
		}
	})
}

// Scheduler finds cards whose due date calls for a reminder (spec §10).
type Scheduler struct {
	Store    *store.Store
	Notifier *Notifier
	Interval time.Duration // default 15m
	Now      func() time.Time
}

// Tick notifies every due card's assignee once per due date and kind.
func (s Scheduler) Tick(ctx context.Context) error {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	candidates, err := s.Store.DueCandidates(ctx, now())
	if err != nil {
		return err
	}
	var events []Event
	for _, c := range candidates {
		events = append(events, Event{
			Kind: c.Kind, To: *c.Card.AssigneeID, Card: c.Card, BoardName: c.BoardName,
			// Per person: a card handed to someone else reminds them as well.
			DedupeKey: c.Kind + ":" + strconv.FormatInt(*c.Card.AssigneeID, 10) + ":" +
				strconv.FormatInt(c.Card.ID, 10) + ":" + c.Card.DueDate.Format(time.DateOnly),
		})
	}
	return s.Notifier.Emit(ctx, events...)
}

// Run ticks until ctx is cancelled.
func (s Scheduler) Run(ctx context.Context) {
	every(ctx, s.Interval, 15*time.Minute, func() {
		if err := s.Tick(ctx); err != nil && ctx.Err() == nil {
			s.Notifier.Log.Error("notify: scheduler", "err", err)
		}
	})
}

func every(ctx context.Context, interval, fallback time.Duration, fn func()) {
	if interval <= 0 {
		interval = fallback
	}
	fn()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			fn()
		}
	}
}
