package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/authz"
	"kanban/internal/config"
	"kanban/internal/files"
	"kanban/internal/store"
)

// cardContext is a card of the board in the URL.
type cardContext struct {
	boardContext
	Card store.Card
}

// cardFor loads the card in the URL, checked against its board: a card of
// another board is not found (spec §6, IDOR).
func (h *handlers) cardFor(ctx context.Context, rc *collage.RenderContext) (cardContext, error) {
	bc, err := h.boardFor(ctx, rc)
	if err != nil {
		return cardContext{}, err
	}
	id, err := strconv.ParseInt(rc.Param("card"), 10, 64)
	if err != nil {
		return cardContext{}, fmt.Errorf("card %q: %w", rc.Param("card"), collage.ErrNotFound)
	}
	card, err := h.store.Card(ctx, bc.Board.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		return cardContext{}, fmt.Errorf("card %d on board %d: %w", id, bc.Board.ID, collage.ErrNotFound)
	}
	if err != nil {
		return cardContext{}, err
	}
	return cardContext{boardContext: bc, Card: card}, nil
}

type cardPageView struct {
	Board store.Board
	Card  store.Card
}

type panelView struct {
	Notices    []string
	Board      store.Board
	Card       store.Card
	Access     authz.BoardAccess
	Archived   bool
	Column     store.Column
	Columns    []store.Column
	Members    []store.Member
	Labels     []panelLabel
	Checklist  []store.ChecklistItem
	Deps       store.Dependencies
	Candidates []store.CardSummary
	Priorities []string

	Comments    []commentView
	Handles     []string
	Attachments []attachmentView
	Activity    []activityView

	Estimate string
	DueDate  string
	Priority string
	Assignee string
}

type commentView struct {
	Comment   store.Comment
	Mine      bool
	CanDelete bool
}

type attachmentView struct {
	Attachment store.Attachment
	Size       string
	CanDelete  bool
}

// maxAttachment is the largest file a card takes (spec §9).
const maxAttachment = 5 << 20

type panelLabel struct {
	Label   store.Label
	Checked bool
}

func (h *handlers) cardPage() *collage.Page {
	h.panel = collage.NewFragment("card-panel", "fragments/card_panel.html").
		WithDataHandler(collage.DataHandler(h.loadPanel)).
		Required().
		Build()
	content := collage.NewFragment("card-content", "pages/card.html").
		WithDataHandler(collage.Load(h.loadCardPage)).
		WithSlotFragment("panel", h.panel).
		Required().
		Build()
	b := paths(h.privatePage("card", content), "/boards/{id}/cards/{card}")
	for _, l := range config.Locales {
		b = b.WithFragmentPath(l, "/boards/{id}/cards/{card}/panel", h.panel)
	}
	// The body limit leaves room for a 5 MB file and the rest of the form, so
	// that the form's own "over 5 MB" message is reached before collage's 413.
	post := collage.NewAction("card-post").WithMethods(http.MethodPost).
		WithMaxBodyBytes(maxAttachment + 64<<10).WithHandler(h.cardPost).Build()
	return b.WithActionFor(post).Dynamic().Build()
}

func (h *handlers) loadCardPage(ctx context.Context, rc *collage.RenderContext) (cardPageView, error) {
	cc, err := h.cardFor(ctx, rc)
	if err != nil {
		return cardPageView{}, err
	}
	rc.HoistTitle(cc.Card.Title)
	return cardPageView{Board: cc.Board, Card: cc.Card}, nil
}

func (h *handlers) loadPanel(ctx context.Context, rc *collage.RenderContext) (panelView, []string, error) {
	cc, err := h.cardFor(ctx, rc)
	if err != nil {
		return panelView{}, nil, err
	}
	card := cc.Card
	tags := []string{cardTag(card.ID), boardTag(cc.Board.ID)}
	v := panelView{
		Board: cc.Board, Card: card, Access: cc.Access, Archived: card.ArchivedAt != nil,
		Priorities: []string{"1", "2", "3", "4"},
	}
	v.Notices, _ = collage.Get[[]string](rc, noticeKey)
	if card.Estimate != nil {
		v.Estimate = strconv.FormatFloat(*card.Estimate, 'f', -1, 64)
	}
	if card.DueDate != nil {
		v.DueDate = card.DueDate.Format(time.DateOnly)
	}
	if card.Priority != nil {
		v.Priority = strconv.Itoa(int(*card.Priority))
	}
	if card.AssigneeID != nil {
		v.Assignee = strconv.FormatInt(*card.AssigneeID, 10)
	}
	if v.Columns, err = h.store.Columns(ctx, cc.Board.ID); err != nil {
		return v, tags, err
	}
	for _, c := range v.Columns {
		if c.ID == card.ColumnID {
			v.Column = c
		}
	}
	if v.Members, err = h.store.Members(ctx, cc.Team.ID); err != nil {
		return v, tags, err
	}
	labels, err := h.store.Labels(ctx, cc.Board.ID)
	if err != nil {
		return v, tags, err
	}
	checked, err := h.store.CardLabels(ctx, card.ID)
	if err != nil {
		return v, tags, err
	}
	for _, l := range labels {
		pl := panelLabel{Label: l}
		for _, id := range checked {
			pl.Checked = pl.Checked || id == l.ID
		}
		v.Labels = append(v.Labels, pl)
	}
	if v.Checklist, err = h.store.ChecklistItems(ctx, card.ID); err != nil {
		return v, tags, err
	}
	if v.Deps, err = h.store.CardDependencies(ctx, card.ID); err != nil {
		return v, tags, err
	}
	all, err := h.store.BoardCards(ctx, cc.Board.ID)
	if err != nil {
		return v, tags, err
	}
	comments, err := h.store.CardComments(ctx, card.ID)
	if err != nil {
		return v, tags, err
	}
	for _, c := range comments {
		mine := c.AuthorID == cc.User.ID
		v.Comments = append(v.Comments, commentView{Comment: c, Mine: mine, CanDelete: mine || cc.Access.CanManage})
	}
	for _, m := range v.Members {
		v.Handles = append(v.Handles, "@"+store.MentionHandle(m.User))
	}
	attachments, err := h.store.CardAttachments(ctx, card.ID)
	if err != nil {
		return v, tags, err
	}
	for _, a := range attachments {
		v.Attachments = append(v.Attachments, attachmentView{Attachment: a, Size: humanSize(a.Size),
			CanDelete: a.UploaderID == cc.User.ID || cc.Access.CanManage})
	}
	activity, err := h.store.CardActivity(ctx, card.ID, 50)
	if err != nil {
		return v, tags, err
	}
	v.Activity = activityViews(rc, activity)
	for _, s := range all {
		taken := s.Card.ID == card.ID
		for _, b := range v.Deps.Blockers {
			taken = taken || b.ID == s.Card.ID
		}
		if !taken {
			v.Candidates = append(v.Candidates, s)
		}
	}
	return v, tags, nil
}

func (h *handlers) cardPost(ctx context.Context, rc *collage.RenderContext) (*collage.ActionResult, error) {
	cc, err := h.cardFor(ctx, rc)
	if errors.Is(err, collage.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if !cc.Access.CanEdit || cc.Card.ArchivedAt != nil {
		return collage.NoContent(http.StatusForbidden), nil
	}
	v := validate.Form(rc)
	switch v.Value("op") {
	case "update":
		return h.updateCard(ctx, rc, v, cc)
	case "labels":
		ids := []int64{}
		for _, raw := range rc.Request.PostForm["label"] {
			if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
				ids = append(ids, id)
			}
		}
		err := h.store.SetCardLabels(ctx, cc.Board.ID, cc.Card.ID, ids)
		if err == nil {
			err = h.store.LogActivity(ctx, cc.Board.ID, &cc.Card.ID, cc.User.ID, store.ActivityLabelsChanged, store.ActivityPayload{})
		}
		return h.cardChanged(rc, cc, err)
	case "checklist_add":
		v.Field("item_text").Required().MaxLen(500)
		if !v.Valid() {
			return h.refuseCard(rc, v), nil
		}
		text := strings.TrimSpace(v.Value("item_text"))
		_, err := h.store.AddChecklistItem(ctx, cc.Board.ID, cc.Card.ID, text)
		if err == nil {
			err = h.store.LogActivity(ctx, cc.Board.ID, &cc.Card.ID, cc.User.ID, store.ActivityChecklistAdded, store.ActivityPayload{Text: text})
		}
		return h.cardChanged(rc, cc, err)
	case "checklist_toggle", "checklist_delete":
		item, ok := formInt64(v, "item_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		if v.Value("op") == "checklist_delete" {
			return h.cardChanged(rc, cc, h.store.DeleteChecklistItem(ctx, cc.Board.ID, cc.Card.ID, item))
		}
		done := v.Value("done") == "1"
		err := h.store.SetChecklistItemDone(ctx, cc.Board.ID, cc.Card.ID, item, done)
		if err == nil && done {
			err = h.logChecklistChecked(ctx, cc, item)
		}
		return h.cardChanged(rc, cc, err)
	case "dep_add", "dep_remove":
		blocker, ok := formInt64(v, "blocker_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		kind := store.ActivityDependencyAdded
		if v.Value("op") == "dep_remove" {
			kind = store.ActivityDependencyRemoved
			err = h.store.RemoveDependency(ctx, cc.Board.ID, blocker, cc.Card.ID)
		} else {
			err = h.store.AddDependency(ctx, cc.Board.ID, blocker, cc.Card.ID)
		}
		if errors.Is(err, store.ErrCycle) {
			return h.cardNotice(rc, cc, http.StatusUnprocessableEntity, i18n.T(rc, "card.cycle"))
		}
		if err == nil {
			if other, lookupErr := h.store.Card(ctx, cc.Board.ID, blocker); lookupErr == nil {
				err = h.store.LogActivity(ctx, cc.Board.ID, &cc.Card.ID, cc.User.ID, kind, store.ActivityPayload{Text: other.Title})
			}
		}
		return h.cardChanged(rc, cc, err)
	case "comment_add", "comment_edit", "comment_delete":
		return h.commentOp(ctx, rc, v, cc)
	case "attachment_add", "attachment_delete":
		return h.attachmentOp(ctx, rc, v, cc)
	case "archive":
		if err := h.store.ArchiveCard(ctx, cc.Board.ID, cc.Card.ID, cc.User.ID); err != nil {
			return nil, err
		}
		h.notifyUnblocked(ctx, cc.boardContext, cc.Card.ID)
		flash.Add(rc, flash.Success, i18n.T(rc, "card.archived"))
		res, err := h.redirectTo(rc, "board", "id", strconv.FormatInt(cc.Board.ID, 10))
		if res != nil {
			res.InvalidateTags = []string{boardTag(cc.Board.ID), cardTag(cc.Card.ID)}
		}
		return res, err
	}
	return collage.NoContent(http.StatusBadRequest), nil
}

// cardChanged answers a change to the card: the panel for a script, a
// redirect back to the card otherwise. Both invalidate the board and the card.
func (h *handlers) cardChanged(rc *collage.RenderContext, cc cardContext, err error) (*collage.ActionResult, error) {
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	res, err := h.cardNotice(rc, cc, http.StatusOK)
	if res != nil {
		res.InvalidateTags = []string{boardTag(cc.Board.ID), cardTag(cc.Card.ID)}
	}
	return res, err
}

// cardNotice answers with the panel carrying notice, or redirects to the card
// with notice as a flash message.
func (h *handlers) cardNotice(rc *collage.RenderContext, cc cardContext, status int, notices ...string) (*collage.ActionResult, error) {
	if isFetch(rc) {
		rc.Set(noticeKey, notices)
		res := collage.RenderFragment(h.panel)
		res.Status = status
		return res, nil
	}
	kind := flash.Success
	if status >= 400 {
		kind = flash.Error
	}
	for _, n := range notices {
		if n != "" {
			flash.Add(rc, kind, n)
		}
	}
	return h.redirectTo(rc, "card", "id", strconv.FormatInt(cc.Board.ID, 10), "card", strconv.FormatInt(cc.Card.ID, 10))
}

// refuseCard answers a form the panel sent that did not validate: the panel
// for a script, the card page otherwise, both with status 422.
func (h *handlers) refuseCard(rc *collage.RenderContext, v *validate.Validator) *collage.ActionResult {
	if isFetch(rc) {
		res := validate.Refuse(rc, v, nil)
		res.Page, res.Fragment = nil, h.panel
		return res
	}
	return validate.Refuse(rc, v, rc.Page)
}

func (h *handlers) updateCard(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, cc cardContext) (*collage.ActionResult, error) {
	var fields store.CardFields
	v.Field("title").Required().MaxLen(200)
	v.Field("description").MaxLen(10000)
	v.Field("priority").OneOf("1", "2", "3", "4")
	v.Field("estimate").Custom(func(s string) string {
		if s == "" {
			return ""
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
		if err != nil || f < 0 {
			return i18n.T(rc, "card.estimate_invalid")
		}
		fields.Estimate = &f
		return ""
	})
	v.Field("due_date").Custom(func(s string) string {
		if s == "" {
			return ""
		}
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			return i18n.T(rc, "card.date_invalid")
		}
		fields.DueDate = &d
		return ""
	})
	members, err := h.store.Members(ctx, cc.Team.ID)
	if err != nil {
		return nil, err
	}
	v.Field("assignee_id").Custom(func(s string) string {
		if s == "" {
			return ""
		}
		for _, m := range members {
			if strconv.FormatInt(m.User.ID, 10) == s {
				id := m.User.ID
				fields.AssigneeID = &id
				return ""
			}
		}
		return i18n.T(rc, "card.assignee_invalid")
	})
	version, ok := formInt64(v, "expected_version")
	if !ok {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	if !v.Valid() {
		return h.refuseCard(rc, v), nil
	}
	fields.Title = strings.TrimSpace(v.Value("title"))
	fields.Description = strings.TrimSpace(v.Value("description"))
	if p := v.Value("priority"); p != "" {
		n, _ := strconv.Atoi(p)
		prio := int16(n)
		fields.Priority = &prio
	}
	updated, err := h.store.UpdateCard(ctx, cc.Board.ID, cc.Card.ID, int(version), fields, cc.User.ID)
	if err == nil {
		h.notifyAssigned(ctx, cc.boardContext, cc.Card, updated)
	}
	if msgs := violationMessages(rc, err); msgs != nil {
		return h.cardNotice(rc, cc, http.StatusUnprocessableEntity, msgs...)
	}
	if errors.Is(err, store.ErrConflict) {
		// The panel shows the card as it now is; what the reader typed is lost,
		// which the notice says. collage-live puts a form's answer in only on
		// success or 422, so a script is told 422 rather than 409.
		status := http.StatusConflict
		if isFetch(rc) {
			status = http.StatusUnprocessableEntity
		}
		return h.cardNotice(rc, cc, status, i18n.T(rc, "board.conflict"))
	}
	if err != nil {
		return nil, err
	}
	res, err := h.cardNotice(rc, cc, http.StatusOK, i18n.T(rc, "card.saved"))
	if res != nil {
		res.InvalidateTags = []string{boardTag(cc.Board.ID), cardTag(cc.Card.ID)}
	}
	return res, err
}

func (h *handlers) logChecklistChecked(ctx context.Context, cc cardContext, item int64) error {
	items, err := h.store.ChecklistItems(ctx, cc.Card.ID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.ID == item {
			return h.store.LogActivity(ctx, cc.Board.ID, &cc.Card.ID, cc.User.ID, store.ActivityChecklistChecked, store.ActivityPayload{Text: it.Text})
		}
	}
	return nil
}

func (h *handlers) commentOp(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, cc cardContext) (*collage.ActionResult, error) {
	members, err := h.store.Members(ctx, cc.Team.ID)
	if err != nil {
		return nil, err
	}
	switch v.Value("op") {
	case "comment_add":
		v.Field("comment").Required().MaxLen(10000)
		if !v.Valid() {
			return h.refuseCard(rc, v), nil
		}
		body := strings.TrimSpace(v.Value("comment"))
		mentions := store.ResolveMentions(body, members)
		_, err = h.store.AddComment(ctx, cc.Board.ID, cc.Card.ID, cc.User.ID, body, mentions)
		if err == nil {
			h.notifyComment(ctx, cc.boardContext, cc.Card, body, mentions)
		}
	case "comment_edit":
		id, ok := formInt64(v, "comment_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		v.Field("comment_body").Required().MaxLen(10000)
		if !v.Valid() {
			return h.refuseCard(rc, v), nil
		}
		body := strings.TrimSpace(v.Value("comment_body"))
		err = h.store.EditComment(ctx, cc.Board.ID, cc.Card.ID, id, cc.User.ID, body, store.ResolveMentions(body, members))
	case "comment_delete":
		id, ok := formInt64(v, "comment_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err = h.store.DeleteComment(ctx, cc.Board.ID, cc.Card.ID, id, cc.User.ID, cc.Access.CanManage)
	}
	return h.cardChanged(rc, cc, err)
}

func (h *handlers) attachmentOp(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, cc cardContext) (*collage.ActionResult, error) {
	if v.Value("op") == "attachment_delete" {
		id, ok := formInt64(v, "attachment_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		key, err := h.store.DeleteAttachment(ctx, cc.Board.ID, cc.Card.ID, id, cc.User.ID, cc.Access.CanManage)
		if err == nil {
			if rmErr := h.files.Remove(key); rmErr != nil {
				h.log.Warn("attachments: remove file", "key", key, "err", rmErr)
			}
		}
		return h.cardChanged(rc, cc, err)
	}
	file, header, err := rc.Request.FormFile("file")
	if err != nil {
		v.Fail("file", i18n.T(rc, "attachments.required"))
		return h.refuseCard(rc, v), nil
	}
	defer file.Close()
	if header.Size > maxAttachment {
		v.Fail("file", i18n.T(rc, "attachments.too_large"))
		return h.refuseCard(rc, v), nil
	}
	saved, err := h.files.Save(file, maxAttachment)
	if errors.Is(err, files.ErrTooLarge) {
		v.Fail("file", i18n.T(rc, "attachments.too_large"))
		return h.refuseCard(rc, v), nil
	}
	if err != nil {
		return nil, err
	}
	name := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if name == "." || name == "/" || name == "" {
		name = "file"
	}
	_, err = h.store.AddAttachment(ctx, cc.Board.ID, cc.Card.ID, store.Attachment{
		UploaderID: cc.User.ID, Filename: name, ContentType: saved.ContentType, Size: saved.Size, StorageKey: saved.Key,
	})
	if err != nil {
		h.files.Remove(saved.Key)
	}
	return h.cardChanged(rc, cc, err)
}

// humanSize writes a byte count for people.
func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MB"
	case n >= 1<<10:
		return strconv.FormatFloat(float64(n)/(1<<10), 'f', 0, 64) + " KB"
	}
	return strconv.FormatInt(n, 10) + " B"
}
