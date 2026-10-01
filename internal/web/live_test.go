package web_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// A reader with the board open receives the columns again when someone else
// moves a card (spec §14, phase 2 acceptance).
func TestAMoveIsPushedToOpenBoards(t *testing.T) {
	b := newBoardSetup(t)
	c := b.card(t, 0, "Pushed card")
	server := httptest.NewServer(b.h.app.Handler())
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/_live/stream/?f="+b.path+"/columns", nil)
	for _, ck := range b.member.Cookies() {
		req.AddCookie(ck)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("stream = %d", res.StatusCode)
	}
	events := make(chan string, 8)
	go func() {
		scanner := bufio.NewScanner(res.Body)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "data:") {
				events <- line
			}
		}
		close(events)
	}()
	next := func() string {
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatal("the stream closed")
			}
			return e
		case <-ctx.Done():
			t.Fatal("no event within the deadline")
		}
		return ""
	}
	first := next()
	if !strings.Contains(first, "Pushed card") {
		t.Fatalf("first event lacks the card: %s", first)
	}

	if r := b.lead.SubmitFetch(b.path, b.path, moveForm(c, b.cols[1].ID, 0, b.cols[0].ID, c.Version)); r.Status != http.StatusOK {
		t.Fatalf("move = %d", r.Status)
	}
	pushed := next()
	// The card's own element now names the column it went to.
	moved := regexp.MustCompile(`data-card=\\"` + id(c.ID) + `\\" data-version=\\"\d+\\" data-column=\\"` + id(b.cols[1].ID) + `\\"`)
	if moved.MatchString(first) {
		t.Fatalf("the check matches before the move: %s", first)
	}
	if !moved.MatchString(pushed) {
		t.Fatalf("pushed event does not show the move: %s", pushed)
	}
}
