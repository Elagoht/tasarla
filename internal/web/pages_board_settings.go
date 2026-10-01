package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"
	"github.com/jackc/pgx/v5/pgconn"

	"kanban/internal/rules"
	"kanban/internal/store"
)

// labelPalette is the colours a label may have. A fixed palette lets the
// stylesheet colour labels, so no inline style is needed under the CSP.
var labelPalette = []string{"#e03131", "#f08c00", "#2f9e44", "#1971c2", "#7048e8", "#c2255c", "#0c8599", "#495057"}

var settingsTabs = []string{"general", "columns", "labels", "roles", "rules"}

type settingsView struct {
	Tab     string
	Tabs    []string
	Board   store.Board
	Team    store.Team
	Columns []columnRowView
	// ColumnsProblem is what is wrong with a refused table as a whole.
	ColumnsProblem string
	Labels         []store.Label
	Palette        []string

	Members   []store.Member
	Roles     []settingsRole
	Sentences []sentenceView
	Subjects  []string
	Kinds     []string
	AllCols   []store.Column
	// Builders are the sentences a new rule is written in, by kind, with
	// blanks where its fields go.
	Builders map[string][]sentencePart
}

// sentencePart is a piece of a rule sentence: text, or the blank named Slot.
type sentencePart struct {
	Text string
	Slot string
}

// builderKeys are the sentences of the rule builders; their blanks are named
// as in the sentence the rule is listed with.
var builderKeys = map[string]string{
	"permission": "settings.builder.permission",
	"from":       "settings.sentence.from",
	"condition":  "settings.builder.condition",
	"wip":        "settings.sentence.wip",
	"person_wip": "settings.sentence.person_wip",
}

// sentenceParts splits "{column} kolonuna …" into its text and its blanks, so
// each language keeps its own word order around the form's fields.
func sentenceParts(text string) []sentencePart {
	var parts []sentencePart
	for text != "" {
		open := strings.IndexByte(text, '{')
		end := strings.IndexByte(text[max(open, 0):], '}')
		if open < 0 || end < 0 {
			parts = append(parts, sentencePart{Text: text})
			break
		}
		end += open
		if open > 0 {
			parts = append(parts, sentencePart{Text: text[:open]})
		}
		parts = append(parts, sentencePart{Slot: text[open+1 : end]})
		text = text[end+1:]
	}
	return parts
}

// columnRowView is one row of the column table, from the database or from a
// refused submission (then with what was typed and its error).
type columnRowView struct {
	Index   int
	ID      string
	Name    string
	WIP     string
	Person  bool
	Create  bool
	Done    bool
	Delete  bool
	Cards   int
	Problem string
}

type settingsRole struct {
	Role    store.BoardRole
	Members []roleMember
}

type roleMember struct {
	User   store.User
	Member bool
}

type sentenceView struct {
	Key     string
	Text    string
	Warning bool
}

// managedBoardFor is boardFor for the settings page: anyone who may not manage
// the board is told it does not exist.
func (h *handlers) managedBoardFor(ctx context.Context, rc *collage.RenderContext) (boardContext, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return bc, err
	}
	if !bc.Access.CanManage {
		return bc, fmt.Errorf("settings of board %d for user %d: %w", bc.Board.ID, bc.User.ID, collage.ErrNotFound)
	}
	return bc, nil
}

func (h *handlers) boardSettingsPage() *collage.Page {
	content := collage.NewFragment("board-settings-content", "pages/board_settings.html").
		WithDataHandler(collage.Load(h.loadBoardSettings)).
		Required().
		Build()
	return paths(h.privatePage("board-settings", content), "/boards/{id}/settings").
		WithAction(http.MethodPost, h.boardSettingsPost).
		Dynamic().
		Build()
}

// draftKey holds a refused column table for the page to show again.
const draftKey = "columns_draft"

type columnsDraft struct {
	Rows    []columnRowView
	Problem string
}

func (h *handlers) loadBoardSettings(ctx context.Context, rc *collage.RenderContext) (settingsView, error) {
	bc, err := h.managedBoardFor(ctx, rc)
	if err != nil {
		return settingsView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "settings.title") + " · " + bc.Board.Name)
	view := settingsView{Board: bc.Board, Team: bc.Team, Tabs: settingsTabs, Palette: labelPalette,
		Subjects: []string{rules.SubjectAnyMember, rules.SubjectTeamLead, rules.SubjectAssignee}, Kinds: rules.Kinds,
		Builders: map[string][]sentencePart{}}
	for kind, key := range builderKeys {
		view.Builders[kind] = sentenceParts(i18n.T(rc, key))
	}
	view.Tab = rc.Request.URL.Query().Get("tab")
	if !slices.Contains(settingsTabs, view.Tab) {
		view.Tab = "general"
	}
	if view.AllCols, err = h.store.Columns(ctx, bc.Board.ID); err != nil {
		return view, err
	}
	if draft, ok := collage.Get[columnsDraft](rc, draftKey); ok {
		view.Columns, view.ColumnsProblem, view.Tab = draft.Rows, draft.Problem, "columns"
	} else {
		cards, err := h.store.BoardCards(ctx, bc.Board.ID)
		if err != nil {
			return view, err
		}
		for i, c := range view.AllCols {
			row := columnRowView{Index: i, ID: strconv.FormatInt(c.ID, 10), Name: c.Name, Person: c.CountsPersonWIP,
				Create: c.AllowCreate, Done: c.IsDone}
			if c.WIPLimit != nil {
				row.WIP = strconv.Itoa(*c.WIPLimit)
			}
			for _, s := range cards {
				if s.Card.ColumnID == c.ID {
					row.Cards++
				}
			}
			view.Columns = append(view.Columns, row)
		}
	}
	if view.Labels, err = h.store.Labels(ctx, bc.Board.ID); err != nil {
		return view, err
	}
	if view.Members, err = h.store.Members(ctx, bc.Team.ID); err != nil {
		return view, err
	}
	r, err := h.store.BoardRules(ctx, bc.Board.ID)
	if err != nil {
		return view, err
	}
	for _, role := range r.Roles {
		sr := settingsRole{Role: role}
		for _, m := range view.Members {
			sr.Members = append(sr.Members, roleMember{User: m.User, Member: slices.Contains(role.MemberIDs, m.User.ID)})
		}
		view.Roles = append(view.Roles, sr)
	}
	sentences, err := h.store.BoardSentences(ctx, bc.Board.ID)
	if err != nil {
		return view, err
	}
	view.Sentences = sentenceViews(rc, sentences, view.AllCols, r.Roles, view.Labels)
	return view, nil
}

// listOf joins names as a sentence lists them: "A, B and C".
func listOf(rc *collage.RenderContext, items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " " + i18n.T(rc, "settings.and") + " " + items[len(items)-1]
}

// sentenceViews writes each rule as a sentence in the reader's language.
func sentenceViews(rc *collage.RenderContext, ss []store.Sentence, cols []store.Column, roles []store.BoardRole, labels []store.Label) []sentenceView {
	colName := map[int64]string{}
	for _, c := range cols {
		colName[c.ID] = c.Name
	}
	roleName := map[int64]string{}
	for _, r := range roles {
		roleName[r.ID] = r.Name
	}
	labelName := map[int64]string{}
	for _, l := range labels {
		labelName[l.ID] = l.Name
	}
	names := func(ids []int64) string {
		var out []string
		for _, id := range ids {
			out = append(out, colName[id])
		}
		return listOf(rc, out)
	}
	var out []sentenceView
	for _, s := range ss {
		col := colName[s.ColumnID]
		v := sentenceView{Key: s.Key}
		switch s.Kind {
		case store.SentencePermission:
			var who []string
			for _, sub := range s.Subjects {
				who = append(who, i18n.T(rc, "settings.subjects."+sub))
			}
			for _, id := range s.RoleIDs {
				who = append(who, roleName[id])
			}
			if s.FromID != 0 {
				v.Text = i18n.T(rc, "settings.sentence.permission_from", "column", col, "from", colName[s.FromID], "who", listOf(rc, who))
			} else {
				v.Text = i18n.T(rc, "settings.sentence.permission", "column", col, "who", listOf(rc, who))
			}
		case store.SentenceFrom:
			if len(s.Columns) == 0 {
				v.Text, v.Warning = i18n.T(rc, "settings.sentence.from_none", "column", col), true
			} else {
				v.Text = i18n.T(rc, "settings.sentence.from", "column", col, "sources", names(s.Columns))
			}
		case store.SentenceCondition:
			kind := s.ConditionKind
			var ls []string
			for _, id := range s.Params.LabelIDs {
				ls = append(ls, labelName[id])
			}
			if kind == rules.HasLabel && len(ls) == 0 {
				kind = "has_any_label"
			}
			phrase := i18n.T(rc, "settings.phrase."+kind, "labels", strings.Join(ls, ", "), "count", strconv.Itoa(s.Params.Count))
			v.Text = i18n.T(rc, "settings.sentence."+s.Phase, "column", col, "condition", phrase)
		case store.SentenceWIP:
			v.Text = i18n.T(rc, "settings.sentence.wip", "column", col, "limit", strconv.Itoa(s.Limit))
		case store.SentencePersonWIP:
			v.Text = i18n.T(rc, "settings.sentence.person_wip", "columns", names(s.Columns), "limit", strconv.Itoa(s.Limit))
		}
		out = append(out, v)
	}
	return out
}

func (h *handlers) boardSettingsPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	bc, err := h.managedBoardFor(ctx, rc)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	v := validate.Form(rc)
	if badText(rc) {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if res := h.confirmFirst(rc, v); res != nil {
		return res, nil
	}
	boardID := bc.Board.ID
	tab := "general"
	switch v.Value("op") {
	case "rename":
		v.Field("board_name").Required().MaxLen(100)
		if !v.Valid() {
			return validate.Refuse(rc, v, rc.Page), nil
		}
		err = h.store.RenameBoard(ctx, boardID, strings.TrimSpace(v.Value("board_name")))
	case "archive_board":
		if err := h.store.ArchiveBoard(ctx, boardID); err != nil {
			return nil, err
		}
		flash.Add(rc, flash.Success, i18n.T(rc, "settings.archived"))
		res, err := h.redirectTo(rc, "team", "id", strconv.FormatInt(bc.Team.ID, 10))
		if res != nil {
			res.InvalidateTags = []string{boardTag(boardID)}
		}
		return res, err
	case "columns_save":
		return h.saveColumns(ctx, rc, v, bc)
	case "label_add":
		tab = "labels"
		v.Field("label_name").Required().MaxLen(40)
		v.Field("label_color").Required().OneOf(labelPalette...)
		if !v.Valid() {
			return validate.Refuse(rc, v, rc.Page), nil
		}
		_, err = h.store.CreateLabel(ctx, boardID, strings.TrimSpace(v.Value("label_name")), v.Value("label_color"))
		if isUniqueViolation(err) {
			v.Fail("label_name", i18n.T(rc, "settings.label_exists"))
			return validate.Refuse(rc, v, rc.Page), nil
		}
	case "label_delete":
		tab = "labels"
		label, ok := formInt64(v, "label_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err = h.store.DeleteLabel(ctx, boardID, label)
	case "role_add", "role_rename", "role_members", "role_delete":
		return h.roleSettings(ctx, rc, v, bc)
	case "rule_add":
		return h.addRule(ctx, rc, v, bc)
	case "rule_delete":
		err = h.store.DeleteSentence(ctx, boardID, v.Value("key"))
		tab = "rules"
	default:
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return h.settingsDone(rc, bc, tab, flash.Success, i18n.T(rc, "settings.saved"))
}

// settingsDone redirects back to a tab of the settings with a message, and
// invalidates the board, whose columns, labels or rules may have changed.
func (h *handlers) settingsDone(rc *collage.RenderContext, bc boardContext, tab, kind, message string) (*collage.ActionResult, error) {
	flash.Add(rc, kind, message)
	target, err := rc.URL("board-settings", map[string]string{"id": strconv.FormatInt(bc.Board.ID, 10)})
	if err != nil {
		return nil, err
	}
	res := collage.SeeOther(target + "?tab=" + tab)
	res.InvalidateTags = []string{boardTag(bc.Board.ID)}
	return res, nil
}

// saveColumns applies the whole column table, or shows it again with every
// problem and with what was typed (spec §2.3).
func (h *handlers) saveColumns(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	n, err := strconv.Atoi(v.Value("col_count"))
	if err != nil || n < 0 || n > 100 {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	type indexed struct {
		order int
		view  columnRowView
		row   store.ColumnRow
	}
	var items []indexed
	seen := map[string]bool{}
	for i := range n {
		p := "col_" + strconv.Itoa(i) + "_"
		view := columnRowView{
			Index: i, ID: v.Value(p + "id"), Name: strings.TrimSpace(v.Value(p + "name")),
			WIP: strings.TrimSpace(v.Value(p + "wip")), Person: v.Value(p+"person") == "1", Delete: v.Value(p+"delete") == "1",
			Create: v.Value(p+"create") == "1", Done: v.Value(p+"done") == "1",
		}
		if view.ID == "" && view.Name == "" {
			continue // the empty "new column" row
		}
		row := store.ColumnRow{Name: view.Name, CountsPersonWIP: view.Person, Delete: view.Delete,
			AllowCreate: view.Create, IsDone: view.Done}
		if view.ID != "" {
			id, err := strconv.ParseInt(view.ID, 10, 64)
			if err != nil || seen[view.ID] {
				return collage.NoContent(http.StatusBadRequest), nil // the table names each column once
			}
			seen[view.ID] = true
			if err != nil {
				return collage.NoContent(http.StatusBadRequest), nil
			}
			row.ID = id
		}
		if !view.Delete {
			switch {
			case view.Name == "" || len([]rune(view.Name)) > 60:
				view.Problem = i18n.T(rc, "settings.name_required")
			case view.WIP != "":
				limit, ok := boundedCount(view.WIP)
				if !ok {
					view.Problem = i18n.T(rc, "settings.wip_invalid")
				}
				row.WIPLimit = &limit
			}
		}
		order, err := strconv.Atoi(v.Value(p + "order"))
		if err != nil {
			order = i
		}
		items = append(items, indexed{order: order, view: view, row: row})
	}
	sort.SliceStable(items, func(a, b int) bool { return items[a].order < items[b].order })
	var draft columnsDraft
	var rows []store.ColumnRow
	problems := false
	for i, it := range items {
		it.view.Index = i
		problems = problems || it.view.Problem != ""
		draft.Rows = append(draft.Rows, it.view)
		rows = append(rows, it.row)
	}
	if !problems {
		err := h.store.SaveColumns(ctx, bc.Board.ID, rows)
		var ce *store.ColumnsError
		switch {
		case errors.As(err, &ce):
			for _, p := range ce.Rows {
				msg := i18n.T(rc, "settings.column_one_left")
				switch {
				case errors.Is(p.Err, store.ErrColumnNotEmpty):
					msg = i18n.T(rc, "settings.column_not_empty")
				case errors.Is(p.Err, store.ErrInUse) && p.Index >= 0:
					msg = i18n.T(rc, "settings.column_in_use")
				}
				if p.Index >= 0 {
					draft.Rows[p.Index].Problem = msg
				} else {
					draft.Problem = msg // the table as a whole: shown above it
				}
			}
		case errors.Is(err, store.ErrNotFound):
			return collage.NoContent(http.StatusNotFound), nil
		case err != nil:
			return nil, err
		default:
			return h.settingsDone(rc, bc, "columns", flash.Success, i18n.T(rc, "settings.saved"))
		}
	}
	rc.Set(draftKey, draft)
	res := collage.RenderPage(rc.Page)
	res.Status = http.StatusUnprocessableEntity
	return res, nil
}

func (h *handlers) roleSettings(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	boardID := bc.Board.ID
	var err error
	switch v.Value("op") {
	case "role_add", "role_rename":
		v.Field("role_name").Required().MaxLen(40)
		if !v.Valid() {
			return validate.Refuse(rc, v, rc.Page), nil
		}
		name := strings.TrimSpace(v.Value("role_name"))
		if v.Value("op") == "role_add" {
			_, err = h.store.CreateBoardRole(ctx, boardID, name)
		} else {
			role, ok := formInt64(v, "role_id")
			if !ok {
				return collage.NoContent(http.StatusBadRequest), nil
			}
			err = h.store.RenameBoardRole(ctx, boardID, role, name)
		}
		if isUniqueViolation(err) {
			return h.settingsDone(rc, bc, "roles", flash.Error, i18n.T(rc, "settings.role_exists"))
		}
	case "role_members":
		role, ok := formInt64(v, "role_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		var ids []int64
		for _, raw := range rc.Request.PostForm["member"] {
			if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
		err = h.store.SetBoardRoleMembers(ctx, boardID, role, ids)
	case "role_delete":
		role, ok := formInt64(v, "role_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err = h.store.DeleteBoardRole(ctx, boardID, role)
		if errors.Is(err, store.ErrInUse) {
			return h.settingsDone(rc, bc, "roles", flash.Error, i18n.T(rc, "settings.role_in_use"))
		}
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return h.settingsDone(rc, bc, "roles", flash.Success, i18n.T(rc, "settings.saved"))
}

// addRule turns one sentence form into rule rows (spec §2.5). Anything a form
// cannot produce is refused before it reaches the store.
func (h *handlers) addRule(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	boardID := bc.Board.ID
	form := rc.Request.PostForm
	ids := func(name string) []int64 {
		var out []int64
		for _, raw := range form[name] {
			if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
				out = append(out, id)
			}
		}
		return out
	}
	invalid := func() (*collage.ActionResult, error) {
		return h.settingsDone(rc, bc, "rules", flash.Error, i18n.T(rc, "settings.rule_invalid"))
	}
	positive := func(name string) (*int, bool) {
		n, ok := boundedCount(v.Value(name))
		if !ok {
			return nil, false
		}
		return &n, true
	}
	column, hasColumn := formInt64(v, "column")
	var err error
	switch v.Value("sentence") {
	case "permission":
		subjects := form["subject"]
		roles := ids("role")
		if !hasColumn || len(subjects)+len(roles) == 0 {
			return invalid()
		}
		var from *int64
		if f, ok := formInt64(v, "from_column"); ok {
			if f == column {
				return invalid() // from a column into itself is no move
			}
			from = &f
		}
		for _, sub := range subjects {
			if sub == rules.SubjectBoardRole || !slices.Contains(rules.Subjects, sub) {
				return invalid()
			}
		}
		for _, sub := range subjects {
			if err = h.store.AddMovePermission(ctx, boardID, store.MovePermission{ToColumnID: column, FromColumnID: from, Subject: sub}); err != nil {
				break
			}
		}
		for _, role := range roles {
			if err != nil {
				break
			}
			err = h.store.AddMovePermission(ctx, boardID, store.MovePermission{ToColumnID: column, FromColumnID: from, Subject: rules.SubjectBoardRole, BoardRoleID: &role})
		}
	case "from":
		sources := ids("from")
		if !hasColumn || len(sources) == 0 {
			return invalid()
		}
		err = h.store.AddFromSentence(ctx, boardID, column, sources)
	case "condition":
		phase, kind := v.Value("phase"), v.Value("kind")
		if !hasColumn || (phase != rules.PhaseEnter && phase != rules.PhaseExit) || !slices.Contains(rules.Kinds, kind) {
			return invalid()
		}
		c := store.ColumnCondition{ColumnID: column, Phase: phase, Kind: kind}
		switch kind {
		case rules.HasLabel:
			c.Params.LabelIDs = ids("label")
		case rules.MinAttachments:
			n, ok := positive("count")
			if !ok {
				return invalid()
			}
			c.Params.Count = *n
		}
		err = h.store.AddCondition(ctx, boardID, c)
	case "wip":
		limit, ok := positive("limit")
		if !hasColumn || !ok {
			return invalid()
		}
		err = h.store.SetWIPLimit(ctx, boardID, column, limit)
	case "person_wip":
		limit, ok := positive("limit")
		counted := ids("counted")
		if !ok || len(counted) == 0 {
			return invalid()
		}
		err = h.store.SetPersonWIP(ctx, boardID, limit, counted)
	default:
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return h.settingsDone(rc, bc, "rules", flash.Success, i18n.T(rc, "settings.saved"))
}

// isUniqueViolation reports a PostgreSQL unique constraint failure.
func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
