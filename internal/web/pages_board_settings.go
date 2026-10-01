package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
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

type settingsView struct {
	Board   store.Board
	Team    store.Team
	Columns []settingsColumn
	Labels  []store.Label
	Palette []string

	PersonLimit string
	Matrix      []matrixRow
	Members     []store.Member
	Roles       []settingsRole
	Permissions []settingsPermission
	Conditions  []settingsCondition
	Subjects    []string
	Kinds       []string
}

type matrixRow struct {
	From  store.Column
	Cells []matrixCell
}

type matrixCell struct {
	To      store.Column
	Value   string // "from-to"
	Allowed bool
	Self    bool
}

type settingsRole struct {
	Role    store.BoardRole
	Members []roleMember
}

type roleMember struct {
	User   store.User
	Member bool
}

type settingsPermission struct {
	ID      int64
	To      string
	From    string // "" for any column
	Subject string
	Role    string
}

type settingsCondition struct {
	ID     int64
	Column string
	Phase  string
	Kind   string
	Count  int
	Labels string
}

type settingsColumn struct {
	Column store.Column
	Limit  string
	First  bool
	Last   bool
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

func (h *handlers) loadBoardSettings(ctx context.Context, rc *collage.RenderContext) (settingsView, error) {
	bc, err := h.managedBoardFor(ctx, rc)
	if err != nil {
		return settingsView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "settings.title") + " · " + bc.Board.Name)
	cols, err := h.store.Columns(ctx, bc.Board.ID)
	if err != nil {
		return settingsView{}, err
	}
	labels, err := h.store.Labels(ctx, bc.Board.ID)
	if err != nil {
		return settingsView{}, err
	}
	view := settingsView{Board: bc.Board, Team: bc.Team, Labels: labels, Palette: labelPalette,
		Subjects: rules.Subjects, Kinds: rules.Kinds}
	names := map[int64]string{}
	for i, c := range cols {
		sc := settingsColumn{Column: c, First: i == 0, Last: i == len(cols)-1}
		if c.WIPLimit != nil {
			sc.Limit = strconv.Itoa(*c.WIPLimit)
		}
		view.Columns = append(view.Columns, sc)
		names[c.ID] = c.Name
	}
	if bc.Board.PersonWIPLimit != nil {
		view.PersonLimit = strconv.Itoa(*bc.Board.PersonWIPLimit)
	}
	r, err := h.store.BoardRules(ctx, bc.Board.ID)
	if err != nil {
		return view, err
	}
	allowed := map[rules.Transition]bool{}
	for _, t := range r.Transitions {
		allowed[t] = true
	}
	for _, from := range cols {
		row := matrixRow{From: from}
		for _, to := range cols {
			pair := rules.Transition{From: from.ID, To: to.ID}
			row.Cells = append(row.Cells, matrixCell{To: to, Value: fmt.Sprintf("%d-%d", from.ID, to.ID), Allowed: allowed[pair], Self: from.ID == to.ID})
		}
		view.Matrix = append(view.Matrix, row)
	}
	if view.Members, err = h.store.Members(ctx, bc.Team.ID); err != nil {
		return view, err
	}
	roleNames := map[int64]string{}
	for _, role := range r.Roles {
		roleNames[role.ID] = role.Name
		sr := settingsRole{Role: role}
		for _, m := range view.Members {
			sr.Members = append(sr.Members, roleMember{User: m.User, Member: slices.Contains(role.MemberIDs, m.User.ID)})
		}
		view.Roles = append(view.Roles, sr)
	}
	for _, p := range r.Permissions {
		sp := settingsPermission{ID: p.ID, To: names[p.ToColumnID], Subject: p.Subject}
		if p.FromColumnID != nil {
			sp.From = names[*p.FromColumnID]
		}
		if p.BoardRoleID != nil {
			sp.Role = roleNames[*p.BoardRoleID]
		}
		view.Permissions = append(view.Permissions, sp)
	}
	labelNames := map[int64]string{}
	for _, l := range labels {
		labelNames[l.ID] = l.Name
	}
	for _, c := range r.Conditions {
		sc := settingsCondition{ID: c.ID, Column: names[c.ColumnID], Phase: c.Phase, Kind: c.Kind, Count: c.Params.Count}
		var ls []string
		for _, id := range c.Params.LabelIDs {
			ls = append(ls, labelNames[id])
		}
		sc.Labels = strings.Join(ls, ", ")
		view.Conditions = append(view.Conditions, sc)
	}
	return view, nil
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
	boardID := bc.Board.ID
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
	case "column_add":
		v.Field("new_column").Required().MaxLen(60)
		if !v.Valid() {
			return validate.Refuse(rc, v, rc.Page), nil
		}
		_, err = h.store.AddColumn(ctx, boardID, strings.TrimSpace(v.Value("new_column")))
	case "column_update":
		return h.updateColumn(ctx, rc, v, bc)
	case "column_move":
		col, ok := formInt64(v, "column_id")
		dir, _ := strconv.Atoi(v.Value("dir"))
		if !ok || (dir != -1 && dir != 1) {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err = h.store.MoveColumn(ctx, boardID, col, dir)
	case "column_delete":
		col, ok := formInt64(v, "column_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err = h.store.DeleteColumn(ctx, boardID, col)
		if errors.Is(err, store.ErrColumnNotEmpty) {
			return h.settingsDone(rc, bc, flash.Error, i18n.T(rc, "settings.column_not_empty"))
		}
	case "label_add":
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
	case "policy", "transitions", "role_add", "role_members", "role_delete",
		"permission_add", "permission_delete", "condition_add", "condition_delete":
		return h.ruleSettings(ctx, rc, v, bc)
	case "label_delete":
		label, ok := formInt64(v, "label_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err = h.store.DeleteLabel(ctx, boardID, label)
	default:
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return h.settingsDone(rc, bc, flash.Success, i18n.T(rc, "settings.saved"))
}

// settingsDone redirects back to the settings with a message, and invalidates
// the board, whose columns or labels may have changed.
func (h *handlers) settingsDone(rc *collage.RenderContext, bc boardContext, kind, message string) (*collage.ActionResult, error) {
	flash.Add(rc, kind, message)
	res, err := h.redirectTo(rc, "board-settings", "id", strconv.FormatInt(bc.Board.ID, 10))
	if res != nil {
		res.InvalidateTags = []string{boardTag(bc.Board.ID)}
	}
	return res, err
}

// updateColumn saves one column's form. The page has a form per column with
// the same field names, so problems are reported as a flash message rather
// than next to a field (collage-validate's names are per page).
func (h *handlers) updateColumn(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	col, ok := formInt64(v, "column_id")
	if !ok {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	u := store.ColumnUpdate{
		Name:            strings.TrimSpace(v.Value("name")),
		IsDone:          v.Value("is_done") == "1",
		AllowCreate:     v.Value("allow_create") == "1",
		CountsPersonWIP: v.Value("counts_person_wip") == "1",
	}
	if u.Name == "" || len([]rune(u.Name)) > 60 {
		return h.settingsDone(rc, bc, flash.Error, i18n.T(rc, "settings.name_required"))
	}
	if raw := strings.TrimSpace(v.Value("wip_limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return h.settingsDone(rc, bc, flash.Error, i18n.T(rc, "settings.wip_invalid"))
		}
		u.WIPLimit = &n
	}
	err := h.store.UpdateColumn(ctx, bc.Board.ID, col, u)
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return h.settingsDone(rc, bc, flash.Success, i18n.T(rc, "settings.saved"))
}

// isUniqueViolation reports a PostgreSQL unique constraint failure.
func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// ruleSettings handles the rules sections of the settings page (spec §5.1).
// Input a form cannot produce is answered 400; nothing reaches the store.
func (h *handlers) ruleSettings(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	boardID := bc.Board.ID
	form := rc.Request.PostForm
	bad := collage.NoContent(http.StatusBadRequest)
	var err error
	switch v.Value("op") {
	case "policy":
		mode := v.Value("transitions_mode")
		if mode != rules.ModeOpen && mode != rules.ModeRestricted {
			return bad, nil
		}
		var limit *int
		if raw := strings.TrimSpace(v.Value("person_wip_limit")); raw != "" {
			n, convErr := strconv.Atoi(raw)
			if convErr != nil || n <= 0 {
				return h.settingsDone(rc, bc, flash.Error, i18n.T(rc, "settings.wip_invalid"))
			}
			limit = &n
		}
		err = h.store.SetBoardPolicy(ctx, boardID, mode, limit)
	case "transitions":
		var pairs []rules.Transition
		for _, raw := range form["t"] {
			from, to, ok := strings.Cut(raw, "-")
			f, err1 := strconv.ParseInt(from, 10, 64)
			t, err2 := strconv.ParseInt(to, 10, 64)
			if !ok || err1 != nil || err2 != nil {
				return bad, nil
			}
			pairs = append(pairs, rules.Transition{From: f, To: t})
		}
		err = h.store.SetTransitions(ctx, boardID, pairs)
	case "role_add":
		v.Field("role_name").Required().MaxLen(40)
		if !v.Valid() {
			return validate.Refuse(rc, v, rc.Page), nil
		}
		_, err = h.store.CreateBoardRole(ctx, boardID, strings.TrimSpace(v.Value("role_name")))
		if isUniqueViolation(err) {
			v.Fail("role_name", i18n.T(rc, "settings.role_exists"))
			return validate.Refuse(rc, v, rc.Page), nil
		}
	case "role_members":
		role, ok := formInt64(v, "role_id")
		if !ok {
			return bad, nil
		}
		var ids []int64
		for _, raw := range form["member"] {
			if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
		err = h.store.SetBoardRoleMembers(ctx, boardID, role, ids)
	case "role_delete":
		role, ok := formInt64(v, "role_id")
		if !ok {
			return bad, nil
		}
		err = h.store.DeleteBoardRole(ctx, boardID, role)
	case "permission_add":
		to, ok := formInt64(v, "to_column")
		subject := v.Value("subject")
		if !ok || !slices.Contains(rules.Subjects, subject) {
			return bad, nil
		}
		p := store.MovePermission{ToColumnID: to, Subject: subject}
		if from, ok := formInt64(v, "from_column"); ok {
			p.FromColumnID = &from
		}
		if subject == rules.SubjectBoardRole {
			role, ok := formInt64(v, "board_role")
			if !ok {
				return bad, nil
			}
			p.BoardRoleID = &role
		}
		err = h.store.AddMovePermission(ctx, boardID, p)
	case "permission_delete":
		id, ok := formInt64(v, "permission_id")
		if !ok {
			return bad, nil
		}
		err = h.store.DeleteMovePermission(ctx, boardID, id)
	case "condition_add":
		col, ok := formInt64(v, "column_id")
		phase, kind := v.Value("phase"), v.Value("kind")
		if !ok || (phase != rules.PhaseEnter && phase != rules.PhaseExit) || !slices.Contains(rules.Kinds, kind) {
			return bad, nil
		}
		c := store.ColumnCondition{ColumnID: col, Phase: phase, Kind: kind}
		switch kind {
		case rules.HasLabel:
			for _, raw := range form["label"] {
				if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
					c.Params.LabelIDs = append(c.Params.LabelIDs, id)
				}
			}
		case rules.MinAttachments:
			n, convErr := strconv.Atoi(strings.TrimSpace(v.Value("count")))
			if convErr != nil || n <= 0 {
				return h.settingsDone(rc, bc, flash.Error, i18n.T(rc, "settings.count_invalid"))
			}
			c.Params.Count = n
		}
		err = h.store.AddCondition(ctx, boardID, c)
	case "condition_delete":
		id, ok := formInt64(v, "condition_id")
		if !ok {
			return bad, nil
		}
		err = h.store.DeleteCondition(ctx, boardID, id)
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	return h.settingsDone(rc, bc, flash.Success, i18n.T(rc, "settings.saved"))
}
