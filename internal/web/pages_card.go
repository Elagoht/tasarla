package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
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

	// NoticeField is the field the notices are about, shown beside it rather
	// than above the panel; SavedField is the field just saved.
	NoticeField string
	SavedField  string
	// Live counts the comments not deleted, for the Comments tab.
	Live int
	// ChecklistDone counts the items done.
	ChecklistDone int
	// OpenBlockers counts the blocking cards not done yet.
	OpenBlockers int
}

// fieldState is what the panel shows beside one field: the notices about it
// and whether it was just saved.
type fieldState struct {
	Name    string
	Notices []string
	Saved   bool
}

// Field is the state of one field; notices about no field stay above the
// panel (TopNotices).
func (v panelView) Field(name string) fieldState {
	f := fieldState{Name: name, Saved: v.SavedField == name}
	if v.NoticeField == name {
		f.Notices = v.Notices
	}
	return f
}

// TopNotices are the notices not shown beside a field.
func (v panelView) TopNotices() []string {
	if v.NoticeField != "" {
		return nil
	}
	return v.Notices
}

// Keys for the field a panel answer is about (spec §2.2).
const (
	noticeFieldKey = "notice_field"
	savedFieldKey  = "saved_field"
)

type commentView struct {
	Comment   store.Comment
	Mine      bool
	CanDelete bool
	Hue       int
	Initial   string
}

type attachmentView struct {
	Attachment store.Attachment
	Size       string
	CanDelete  bool
}

// maxAttachment is the largest file a card takes (spec §9).
const maxAttachment = 5 << 20

// maxUploadBody is the largest card form body read; past it collage answers 413.
const maxUploadBody = 64 << 20

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
	// The body limit is well above the 5 MB a file may have, so that any file
	// over it is answered with the form's own message rather than collage's
	// bare 413 (spec §9's 5 MB + 64 KB limit left every file between that and
	// a browser's limit with no message). files.Save still refuses past 5 MB.
	post := collage.NewAction("card-post").WithMethods(http.MethodPost).
		WithMaxBodyBytes(maxUploadBody).WithHandler(h.cardPost).Build()
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
	v.NoticeField, _ = collage.Get[string](rc, noticeFieldKey)
	v.SavedField, _ = collage.Get[string](rc, savedFieldKey)
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
	for _, it := range v.Checklist {
		if it.Done {
			v.ChecklistDone++
		}
	}
	if v.Deps, err = h.store.CardDependencies(ctx, card.ID); err != nil {
		return v, tags, err
	}
	for _, b := range v.Deps.Blockers {
		if !b.Done {
			v.OpenBlockers++
		}
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
		v.Comments = append(v.Comments, commentView{Comment: c, Mine: mine, CanDelete: mine || cc.Access.CanManage,
			Hue: int(c.AuthorID % 8), Initial: initial(c.AuthorName)})
		if c.DeletedAt == nil {
			v.Live++
		}
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
	if res := h.confirmFirst(rc, v); res != nil {
		return res, nil
	}
	switch v.Value("op") {
	case "set_field":
		return h.setField(ctx, rc, v, cc)
	case "move_to":
		return h.moveFromPanel(ctx, rc, v, cc)
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
		if err != nil {
			return h.cardChanged(rc, cc, err)
		}
		return h.fieldSaved(rc, cc, "labels")
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

// fieldSaved answers a saved field: the panel marking it saved for a script,
// back to the card with a message otherwise.
func (h *handlers) fieldSaved(rc *collage.RenderContext, cc cardContext, field string) (*collage.ActionResult, error) {
	rc.Set(savedFieldKey, field)
	var msgs []string
	if !isFetch(rc) {
		msgs = append(msgs, i18n.T(rc, "card.saved"))
	}
	res, err := h.cardNotice(rc, cc, http.StatusOK, msgs...)
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
		before := h.commentMentions(ctx, cc.Card.ID, id)
		mentions := store.ResolveMentions(body, members)
		err = h.store.EditComment(ctx, cc.Board.ID, cc.Card.ID, id, cc.User.ID, body, mentions)
		if err == nil {
			var added []int64
			for _, m := range mentions {
				if !slices.Contains(before, m) {
					added = append(added, m)
				}
			}
			h.notifyMentioned(ctx, cc.boardContext, cc.Card, body, added)
		}
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

// commentMentions returns who a comment mentions now.
func (h *handlers) commentMentions(ctx context.Context, cardID, commentID int64) []int64 {
	comments, err := h.store.CardComments(ctx, cardID)
	if err != nil {
		return nil
	}
	for _, c := range comments {
		if c.ID == commentID {
			return c.MentionIDs
		}
	}
	return nil
}

// setField saves one field of the card (spec §2.2): the others keep what the
// card holds now, so nothing typed elsewhere is lost or overwritten.
func (h *handlers) setField(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, cc cardContext) (*collage.ActionResult, error) {
	version, ok := formInt64(v, "expected_version")
	field := store.CardField(v.Value("field"))
	if !ok {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	value := strings.TrimSpace(v.Value("value"))
	var f store.CardFields
	problem := ""
	switch field {
	case store.FieldTitle:
		if value == "" || len([]rune(value)) > 200 {
			problem = i18n.T(rc, "card.title_invalid")
		}
		f.Title = value
	case store.FieldDescription:
		if len([]rune(value)) > 10000 {
			problem = i18n.T(rc, "card.description_invalid")
		}
		f.Description = value
	case store.FieldAssignee:
		if value != "" {
			members, err := h.store.Members(ctx, cc.Team.ID)
			if err != nil {
				return nil, err
			}
			problem = i18n.T(rc, "card.assignee_invalid")
			for _, m := range members {
				if strconv.FormatInt(m.User.ID, 10) == value {
					id := m.User.ID
					f.AssigneeID, problem = &id, ""
				}
			}
		}
	case store.FieldDueDate:
		if value != "" {
			d, err := time.Parse(time.DateOnly, value)
			if err != nil {
				problem = i18n.T(rc, "card.date_invalid")
			}
			f.DueDate = &d
		}
	case store.FieldPriority:
		if value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 4 {
				problem = i18n.T(rc, "card.priority_invalid")
			}
			p := int16(n)
			f.Priority = &p
		}
	case store.FieldEstimate:
		if value != "" {
			e, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
			if err != nil || e < 0 {
				problem = i18n.T(rc, "card.estimate_invalid")
			}
			f.Estimate = &e
		}
	default:
		return collage.NoContent(http.StatusBadRequest), nil
	}
	rc.Set(noticeFieldKey, string(field))
	if problem != "" {
		return h.cardNotice(rc, cc, http.StatusUnprocessableEntity, problem)
	}
	updated, err := h.store.UpdateCardField(ctx, cc.Board.ID, cc.Card.ID, int(version), field, f, cc.User.ID)
	if msgs := violationMessages(rc, err); msgs != nil {
		return h.cardNotice(rc, cc, http.StatusUnprocessableEntity, msgs...)
	}
	if errors.Is(err, store.ErrConflict) {
		return h.cardNotice(rc, cc, http.StatusUnprocessableEntity, i18n.T(rc, "board.conflict"))
	}
	if err != nil {
		return nil, err
	}
	if field == store.FieldAssignee {
		h.notifyAssigned(ctx, cc.boardContext, cc.Card, updated)
	}
	return h.fieldSaved(rc, cc, string(field))
}

// moveFromPanel moves the card to the bottom of another column from its
// panel, through the rules like any move, and answers with the panel.
func (h *handlers) moveFromPanel(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, cc cardContext) (*collage.ActionResult, error) {
	to, ok1 := formInt64(v, "to_column")
	from, ok2 := formInt64(v, "expected_from")
	version, ok3 := formInt64(v, "expected_version")
	if !ok1 || !ok2 || !ok3 {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	rc.Set(noticeFieldKey, "column")
	moved, err := h.store.MoveCard(ctx, store.Move{
		BoardID: cc.Board.ID, CardID: cc.Card.ID, ToColumnID: to, ToIndex: 1 << 30,
		ExpectedFrom: from, ExpectedVersion: int(version), Actor: cc.actor(),
	})
	if msgs := violationMessages(rc, err); msgs != nil {
		return h.cardNotice(rc, cc, http.StatusUnprocessableEntity, msgs...)
	}
	if errors.Is(err, store.ErrConflict) {
		return h.cardNotice(rc, cc, http.StatusUnprocessableEntity, i18n.T(rc, "board.conflict"))
	}
	if errors.Is(err, store.ErrNotFound) {
		return collage.NoContent(http.StatusNotFound), nil
	}
	if err != nil {
		return nil, err
	}
	if moved.ColumnID != from && h.finishedBy(ctx, moved) {
		h.notifyUnblocked(ctx, cc.boardContext, moved.ID)
	}
	return h.fieldSaved(rc, cc, "column")
}
