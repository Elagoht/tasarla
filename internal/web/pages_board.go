package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/authz"
	"kanban/internal/config"
	"kanban/internal/store"
)

// boardContext is a board and what the signed-in user may do on it, loaded
// once per render.
type boardContext struct {
	User   store.User
	Board  store.Board
	Team   store.Team
	Role   store.Role
	Access authz.BoardAccess
}

// boardFor loads the board in the URL. A board the user may not see, or an
// archived one, is reported as not found (spec §6).
func (h *handlers) boardFor(ctx context.Context, rc *collage.RenderContext) (boardContext, error) {
	return collage.Once(rc, "board:"+rc.Param("id"), func(ctx context.Context) (boardContext, error) {
		user, err := currentUser(ctx)
		if err != nil {
			return boardContext{}, err
		}
		id, err := parseID(rc.Param("id"))
		if err != nil {
			return boardContext{}, fmt.Errorf("board %q: %w", rc.Param("id"), collage.ErrNotFound)
		}
		board, err := h.store.Board(ctx, id)
		if errors.Is(err, store.ErrNotFound) || (err == nil && board.ArchivedAt != nil) {
			return boardContext{}, fmt.Errorf("board %d: %w", id, collage.ErrNotFound)
		}
		if err != nil {
			return boardContext{}, err
		}
		team, err := h.store.Team(ctx, board.TeamID)
		if err != nil {
			return boardContext{}, err
		}
		role, err := h.store.MemberRole(ctx, team.ID, user.ID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return boardContext{}, err
		}
		access := authz.Board(user, role)
		if !access.CanView {
			return boardContext{}, fmt.Errorf("board %d for user %d: %w", id, user.ID, collage.ErrNotFound)
		}
		return boardContext{User: user, Board: board, Team: team, Role: role, Access: access}, nil
	})
}

func boardTag(id int64) string { return "board:" + strconv.FormatInt(id, 10) }
func cardTag(id int64) string  { return "card:" + strconv.FormatInt(id, 10) }

// isFetch reports whether a script sent the request (board.js, collage-live).
func isFetch(rc *collage.RenderContext) bool { return rc.Request.Header.Get(collage.FetchHeader) != "" }

// noticeKey holds the messages ([]string) for the fragment or page an action
// answers with.
const noticeKey = "notices"

type boardView struct {
	Notices []string
	Board   store.Board
	Team    store.Team
	Access  authz.BoardAccess
	Filter  filterView
}

type columnsView struct {
	Notices []string
	CanEdit bool
	Columns []columnView
	Filter  filterView
	// Matching of Total cards match the filter; MoveURL is the board page
	// with the filter, where moves and new cards are sent.
	Matching, Total int
	MoveURL         string
	// Lane is the field the board is split by ("" for none); Lanes are its
	// lanes, made only then.
	Lane  string
	Lanes []laneView
	// Templates are offered in every add-card form.
	Templates []store.Template
}

type filterView struct {
	Filter  boardFilter
	Query   string // Filter.Query()
	Action  string // the page the bar submits to, without a query
	Members []store.Member
	Labels  []store.Label
	Extra   url.Values // other settings the page keeps (Gantt's scale…), as hidden fields
	// ShowLane offers the lane choice: the board has lanes, the Gantt chart not.
	ShowLane bool
}

// boardFilterFor is the board's filter for this render — the page's and its
// columns' — read from the request's own URL, so a fragment pushed or answered
// to an action keeps the filter its URL carries.
func (h *handlers) boardFilterFor(ctx context.Context, rc *collage.RenderContext, bc boardContext) (filterView, error) {
	return collage.Once(rc, "filter:"+rc.Param("id"), func(ctx context.Context) (filterView, error) {
		members, err := h.store.Members(ctx, bc.Team.ID)
		if err != nil {
			return filterView{}, err
		}
		labels, err := h.store.Labels(ctx, bc.Board.ID)
		if err != nil {
			return filterView{}, err
		}
		action, err := h.urlIn("board", rc.Locale, map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)})
		if err != nil {
			return filterView{}, err
		}
		f := parseBoardFilter(rc.Request.URL.Query(), members, labels)
		return filterView{Filter: f, Query: f.Query(), Action: action, Members: members, Labels: labels}, nil
	})
}

type columnView struct {
	Column store.Column
	Color  string
	Limit  int
	// State is how full the column is against its limit: ok, full or over.
	State     string
	OverLimit bool
	// CanCreate: a card may be added here; FirstCreate marks the first such
	// column, where a refused title without a column is shown again.
	CanCreate   bool
	FirstCreate bool
	Cards       []cardView
}

type cardView struct {
	Summary  store.CardSummary
	Due      string
	Overdue  bool
	Priority string
	Hue      int
	Initial  string
	Dimmed   bool
}

func (h *handlers) boardPage() *collage.Page {
	h.columns = collage.NewFragment("board-columns", "fragments/columns.html").
		WithDataHandler(collage.DataHandler(h.loadColumns)).
		Required().
		Build()
	content := collage.NewFragment("board-content", "pages/board.html").
		WithDataHandler(collage.Load(h.loadBoard)).
		WithSlotFragment("columns", h.columns).
		Required().
		Build()
	b := paths(h.privatePage("board", content), "/boards/{id}")
	for _, l := range config.Locales {
		b = b.WithFragmentPath(l, "/boards/{id}/columns", h.columns)
	}
	return b.WithAction(http.MethodPost, h.boardPost).Dynamic().Build()
}

func (h *handlers) loadBoard(ctx context.Context, rc *collage.RenderContext) (boardView, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return boardView{}, err
	}
	rc.HoistTitle(bc.Board.Name)
	notices, _ := collage.Get[[]string](rc, noticeKey)
	fv, err := h.boardFilterFor(ctx, rc, bc)
	if err != nil {
		return boardView{}, err
	}
	fv.ShowLane = true
	return boardView{Notices: notices, Board: bc.Board, Team: bc.Team, Access: bc.Access, Filter: fv}, nil
}

func (h *handlers) loadColumns(ctx context.Context, rc *collage.RenderContext) (columnsView, []string, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return columnsView{}, nil, err
	}
	tags := []string{boardTag(bc.Board.ID)}
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return columnsView{}, tags, err
	}
	cards, err := h.store.BoardCards(ctx, bc.Board.ID)
	if err != nil {
		return columnsView{}, tags, err
	}
	fv, err := h.boardFilterFor(ctx, rc, bc)
	if err != nil {
		return columnsView{}, tags, err
	}
	today := time.Now().In(h.loc)
	matching, err := h.store.MatchingCardIDs(ctx, bc.Board.ID, fv.Filter.Store(bc.User.ID, today))
	if err != nil {
		return columnsView{}, tags, err
	}
	view := columnsView{CanEdit: bc.Access.CanEdit, Filter: fv, MoveURL: withQuery(fv.Action, fv.Query)}
	if bc.Access.CanEdit {
		// Every render asks again (a push too); the list is small.
		if view.Templates, err = h.store.Templates(ctx, bc.Board.ID); err != nil {
			return columnsView{}, tags, err
		}
	}
	view.Notices, _ = collage.Get[[]string](rc, noticeKey)
	byColumn := map[int64]int{}
	creatable := map[int64]bool{}
	if len(cols) > 0 {
		for _, c := range creatableColumns(cols) {
			creatable[c.ID] = true
		}
	}
	first := true
	for i, c := range cols {
		cv := columnView{Column: c, Color: boardColor(i), CanCreate: bc.Access.CanEdit && creatable[c.ID]}
		cv.FirstCreate, first = cv.CanCreate && first, first && !cv.CanCreate
		if c.WIPLimit != nil {
			cv.Limit = *c.WIPLimit
		}
		byColumn[c.ID] = len(view.Columns)
		view.Columns = append(view.Columns, cv)
	}
	todayISO := today.Format(time.DateOnly)
	for _, s := range cards {
		i, ok := byColumn[s.Card.ColumnID]
		if !ok {
			continue
		}
		cv := cardView{Summary: s, Initial: initial(s.AssigneeName)}
		view.Total++
		if matching == nil || matching[s.Card.ID] {
			view.Matching++
		} else {
			cv.Dimmed = true
		}
		if s.Card.AssigneeID != nil {
			cv.Hue = int(*s.Card.AssigneeID % 8)
		}
		if s.Card.DueDate != nil {
			cv.Due = s.Card.DueDate.Format(time.DateOnly)
			cv.Overdue = cv.Due < todayISO && !view.Columns[i].Column.IsDone
		}
		if s.Card.Priority != nil {
			cv.Priority = i18n.T(rc, "card.priorities."+strconv.Itoa(int(*s.Card.Priority)))
		}
		view.Columns[i].Cards = append(view.Columns[i].Cards, cv)
	}
	for i := range view.Columns {
		c := &view.Columns[i]
		c.OverLimit = c.Limit > 0 && len(c.Cards) > c.Limit
		switch {
		case c.OverLimit:
			c.State = "over"
		case c.Limit > 0 && len(c.Cards) == c.Limit:
			c.State = "full"
		default:
			c.State = "ok"
		}
	}
	if fv.Filter.Lane != "" {
		view.Lane = fv.Filter.Lane
		view.Lanes = buildLanes(view.Lane, view.Columns, fv.Members, i18n.T(rc, "board.unassigned"), i18n.T(rc, "lanes.no_priority"),
			func(p int16) string { return i18n.T(rc, "card.priorities."+strconv.Itoa(int(p))) })
		doneHint, emptyHint := i18n.T(rc, "done.drop_here"), i18n.T(rc, "board.no_cards")
		for i := range view.Lanes {
			for j := range view.Lanes[i].Cells {
				cell := &view.Lanes[i].Cells[j]
				cell.Empty = emptyHint
				if cell.Done {
					cell.Empty = doneHint
				}
			}
		}
	}
	return view, tags, nil
}

func (h *handlers) boardPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	bc, err := h.boardFor(ctx, rc)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if !bc.Access.CanEdit {
		return collage.NoContent(http.StatusForbidden), nil
	}
	v := validate.Form(rc)
	if badText(rc) {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	switch v.Value("op") {
	case "create_card":
		return h.createCard(ctx, rc, v, bc)
	case "move":
		return h.moveCard(ctx, rc, v, bc)
	}
	return collage.NoContent(http.StatusBadRequest), nil
}

func (h *handlers) createCard(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	// A template gives the card its title, so a form that chose one may leave
	// the title empty; a title typed all the same is the card's.
	templateID, err := templateChoice(v)
	if err != nil {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if templateID == 0 {
		v.Field("title").Required()
	}
	v.Field("title").MaxLen(200)
	if !v.Valid() {
		res := validate.Refuse(rc, v, rc.Page)
		if isFetch(rc) {
			// board.js sends the form: the columns, with the title and its error.
			res.Page, res.Fragment = nil, h.columns
		}
		return res, nil
	}
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return collage.NoContent(http.StatusConflict), nil
	}
	// A card goes into the column it was added in, which must be one cards
	// are made in; without one, into the first such column.
	creatable := creatableColumns(cols)
	target := creatable[0]
	if raw := v.Value("column"); raw != "" {
		found := false
		for _, c := range creatable {
			if strconv.FormatInt(c.ID, 10) == raw {
				target, found = c, true
			}
		}
		if !found {
			return collage.NoContent(http.StatusBadRequest), nil
		}
	}
	title := strings.TrimSpace(v.Value("title"))
	var made store.FromTemplate
	if templateID != 0 {
		templates, terr := h.store.Templates(ctx, bc.Board.ID)
		if terr != nil {
			return nil, terr
		}
		if !slices.ContainsFunc(templates, func(t store.Template) bool { return t.ID == templateID }) {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		made, err = h.store.CreateCardFromTemplate(ctx, bc.Board.ID, templateID, target.ID, title, bc.User.ID, h.loc, time.Now())
	} else {
		_, err = h.store.CreateCard(ctx, bc.Board.ID, target.ID, title, bc.User.ID)
	}
	if msgs := violationMessages(rc, err); msgs != nil {
		rc.Set(noticeKey, msgs)
		res := collage.RenderPage(rc.Page)
		if isFetch(rc) {
			res = collage.RenderFragment(h.columns)
		}
		res.Status = http.StatusUnprocessableEntity
		return res, nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusBadRequest), nil // the template or the column was deleted meanwhile
	}
	if err != nil {
		return nil, err
	}
	h.notifyAssigned(ctx, bc, store.Card{}, made.Card)
	dropped := made.AssigneeDropped
	if isFetch(rc) {
		if dropped {
			rc.Set(noticeKey, []string{i18n.T(rc, "board.template_assignee_dropped")})
		}
		res := collage.RenderFragment(h.columns)
		res.InvalidateTags = []string{boardTag(bc.Board.ID)}
		return res, nil
	}
	if dropped {
		flash.Add(rc, flash.Warning, i18n.T(rc, "board.template_assignee_dropped"))
	}
	res, err := h.redirectTo(rc, "board", "id", strconv.FormatInt(bc.Board.ID, 10))
	if res != nil {
		res.InvalidateTags = []string{boardTag(bc.Board.ID)}
	}
	return res, err
}

// templateChoice is the template a form chose: 0 for none, an error for a
// value that is not an id.
func templateChoice(v *validate.Validator) (int64, error) {
	raw := strings.TrimSpace(v.Value("template"))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, errors.New("template: not an id")
	}
	return n, nil
}

// creatableColumns are the columns a card is made in: those marked so, or the
// first column when none is (spec §5). cols must not be empty.
func creatableColumns(cols []store.Column) []store.Column {
	var out []store.Column
	for _, c := range cols {
		if c.AllowCreate {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		out = cols[:1]
	}
	return out
}

// formInt64 reads a whole number from the form; ok is false when it is missing
// or malformed.
func formInt64(v *validate.Validator, field string) (int64, bool) {
	n, err := strconv.ParseInt(v.Value(field), 10, 64)
	return n, err == nil
}

func (h *handlers) moveCard(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	cardID, ok1 := formInt64(v, "card")
	to, ok2 := formInt64(v, "to_column")
	from, ok3 := formInt64(v, "expected_from")
	version, ok4 := formInt64(v, "expected_version")
	index, err := strconv.Atoi(v.Value("to_index"))
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if err != nil {
		index = 1 << 30 // the fallback form sends none: the bottom of the column
	}
	m := store.Move{
		BoardID: bc.Board.ID, CardID: cardID, ToColumnID: to, ToIndex: index,
		ExpectedFrom: from, ExpectedVersion: int(version), Actor: bc.actor(),
	}
	if lane := v.Value("lane"); lane != "" {
		target, ok := laneTarget(lane, v.Value("lane_value"), v.Value("before_card_id"))
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		m.Lane = &target
	}
	mr, err := h.store.MoveCardWith(ctx, m)
	moved := mr.After
	status, notices := http.StatusOK, violationMessages(rc, err)
	switch {
	case notices != nil:
		status = http.StatusUnprocessableEntity
	case errors.Is(err, store.ErrConflict):
		status, notices = http.StatusConflict, []string{i18n.T(rc, "board.conflict")}
	case errors.Is(err, store.ErrNotFound):
		return collage.NoContent(http.StatusNotFound), nil
	case err != nil:
		return nil, err
	}
	tags := []string{boardTag(bc.Board.ID), cardTag(cardID)}
	if err == nil && m.Lane != nil {
		h.notifyAssigned(ctx, bc, mr.Before, mr.After)
	}
	if err == nil && moved.ColumnID != from && h.finishedBy(ctx, moved) {
		h.notifyUnblocked(ctx, bc, moved.ID)
	}
	if isFetch(rc) {
		rc.Set(noticeKey, notices)
		res := collage.RenderFragment(h.columns)
		res.Status = status
		if status == http.StatusOK {
			res.InvalidateTags = tags
		}
		return res, nil
	}
	if len(notices) > 0 {
		for _, n := range notices {
			flash.Add(rc, flash.Error, n)
		}
	} else {
		flash.Add(rc, flash.Success, i18n.T(rc, "board.moved"))
	}
	res, err := h.redirectTo(rc, "card", "id", strconv.FormatInt(bc.Board.ID, 10), "card", strconv.FormatInt(cardID, 10))
	if res != nil && status == http.StatusOK {
		res.InvalidateTags = tags
	}
	return res, err
}
