package web

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type searchView struct {
	Query  string
	Ran    bool // the query was long enough to run
	Groups []searchGroup
	Page   int
	More   bool
}

type searchGroup struct {
	BoardID   int64
	BoardName string
	TeamName  string
	Hits      []searchHitView
}

type searchHitView struct {
	Hit     store.SearchHit
	Title   highlighted
	Excerpt highlighted
	Done    bool
}

func (h *handlers) searchPage() *collage.Page {
	content := collage.NewFragment("search-content", "pages/search.html").
		WithData(collage.Load(h.loadSearch)).
		Required().
		Build()
	return paths(h.privatePage("search", content), "/search").Dynamic().Build()
}

func (h *handlers) loadSearch(ctx context.Context, rc *collage.RenderContext) (searchView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return searchView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "search.title"))
	q := rc.Request.URL.Query()
	v := searchView{Query: strings.TrimSpace(q.Get("q")), Page: 1}
	if p, err := strconv.Atoi(q.Get("page")); err == nil && p > 1 {
		v.Page = min(p, store.MaxSearchPage)
	}
	hits, more, err := h.store.Search(ctx, user.ID, v.Query, v.Page)
	if err != nil {
		return v, err
	}
	v.Ran, v.More = utf8.RuneCountInString(v.Query) >= 2, more
	// Grouped by board, in the order the boards' best hits came.
	at := map[int64]int{}
	for _, hit := range hits {
		i, ok := at[hit.Card.BoardID]
		if !ok {
			i = len(v.Groups)
			at[hit.Card.BoardID] = i
			v.Groups = append(v.Groups, searchGroup{BoardID: hit.Card.BoardID, BoardName: hit.BoardName, TeamName: hit.TeamName})
		}
		hv := searchHitView{Hit: hit, Title: highlight(hit.Card.Title, v.Query, 0), Done: hit.Card.CompletedAt != nil}
		if hit.Excerpt != "" {
			hv.Excerpt = highlight(hit.Excerpt, v.Query, 160)
		}
		v.Groups[i].Hits = append(v.Groups[i].Hits, hv)
	}
	return v, nil
}
