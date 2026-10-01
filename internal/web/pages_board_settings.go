package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"
	"github.com/jackc/pgx/v5/pgconn"

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
	return paths(privatePage("board-settings", content), "/boards/{id}/settings").
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
	view := settingsView{Board: bc.Board, Team: bc.Team, Labels: labels, Palette: labelPalette}
	for i, c := range cols {
		sc := settingsColumn{Column: c, First: i == 0, Last: i == len(cols)-1}
		if c.WIPLimit != nil {
			sc.Limit = strconv.Itoa(*c.WIPLimit)
		}
		view.Columns = append(view.Columns, sc)
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
