package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	flash "github.com/Elagoht/collage-flash"
	i18n "github.com/Elagoht/collage-i18n"
	validate "github.com/Elagoht/collage-validate"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

// scheduleKinds are the choices of a template's schedule; "" is none.
var scheduleKinds = []string{"", "daily", "weekly", "monthly"}

// The bounds of a template's checklist.
const (
	maxChecklistItems = 50
	maxChecklistText  = 200
)

// timeOfDay is a schedule's time as a time input sends it: "09:30".
var timeOfDay = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// templateDraftKey holds a refused template form for the page to show again.
const templateDraftKey = "template_draft"

// templateView is one template of the list.
type templateView struct {
	ID       int64
	Name     string
	Schedule string // the schedule kind, "" for none
	// NoColumn: the target column was deleted, so a schedule cannot run.
	NoColumn bool
	Run      *templateRunView
}

// templateRunView is what the latest run of a schedule did.
type templateRunView struct {
	Failed   bool
	Messages []string
}

// templateForm is the template form: what was stored, or what was posted and
// refused, with each field's error.
type templateForm struct {
	ID          string // "" for a new template
	Name        string
	Title       string
	Description string
	Priority    string
	Assignee    string
	Column      string
	DueInDays   string
	Labels      []int64
	Checklist   string
	Kind        string
	Weekdays    uint8 // bit 0 Monday … bit 6 Sunday
	MonthDay    string
	Time        string
	Errors      map[string]string
}

// weekdayChoice is one day of the weekly schedule's checkboxes; N 0 is Monday.
type weekdayChoice struct {
	N       int
	Checked bool
}

// Days are the seven weekday checkboxes, Monday first.
func (f templateForm) Days() []weekdayChoice {
	days := make([]weekdayChoice, 7)
	for n := range days {
		days[n] = weekdayChoice{N: n, Checked: f.Weekdays&(1<<n) != 0}
	}
	return days
}

// HasLabel reports whether the label is ticked.
func (f templateForm) HasLabel(id int64) bool { return slices.Contains(f.Labels, id) }

// Error is the field's message, or "".
func (f templateForm) Error(field string) string { return f.Errors[field] }

// Kinds are the schedule choices.
func (f templateForm) Kinds() []string { return scheduleKinds }

// newTemplateForm is the form of a template not saved yet.
func newTemplateForm() templateForm {
	return templateForm{MonthDay: "1", Time: "09:00"}
}

// templateFormOf is the form of a stored template.
func templateFormOf(t store.Template) templateForm {
	f := templateForm{
		ID: strconv.FormatInt(t.ID, 10), Name: t.Name, Title: t.Title, Description: t.Description,
		Labels: t.LabelIDs, Checklist: strings.Join(t.Checklist, "\n"), Kind: t.Schedule.Kind,
		Weekdays: t.Schedule.Weekdays, MonthDay: strconv.Itoa(t.Schedule.MonthDay),
		Time: fmt.Sprintf("%02d:%02d", t.Schedule.Hour, t.Schedule.Minute),
	}
	if t.Priority != nil {
		f.Priority = strconv.Itoa(int(*t.Priority))
	}
	if t.AssigneeID != nil {
		f.Assignee = strconv.FormatInt(*t.AssigneeID, 10)
	}
	if t.ColumnID != nil {
		f.Column = strconv.FormatInt(*t.ColumnID, 10)
	}
	if t.DueInDays != nil {
		f.DueInDays = strconv.Itoa(*t.DueInDays)
	}
	return f
}

// loadTemplatesTab fills the templates tab: the list, and the form when
// ?edit= names a template or "new".
func (h *handlers) loadTemplatesTab(ctx context.Context, rc *collage.RenderContext, view *settingsView) error {
	ts, err := h.store.Templates(ctx, view.Board.ID)
	if err != nil {
		return err
	}
	for _, t := range ts {
		tv := templateView{ID: t.ID, Name: t.Name, Schedule: t.Schedule.Kind, NoColumn: t.ColumnID == nil}
		if r := t.LastRun; r != nil {
			tv.Run = &templateRunView{Failed: r.Status == "failed", Messages: ruleMessages(rc, r.Violations)}
		}
		view.Templates = append(view.Templates, tv)
	}
	if draft, ok := collage.Get[templateForm](rc, templateDraftKey); ok {
		view.EditTemplate = &draft
		return nil
	}
	switch edit := rc.Request.URL.Query().Get("edit"); edit {
	case "":
	case "new":
		f := newTemplateForm()
		view.EditTemplate = &f
	default:
		id, err := parseID(edit)
		if err != nil {
			return fmt.Errorf("template %q: %w", edit, collage.ErrNotFound)
		}
		t, err := h.store.Template(ctx, view.Board.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("template %d of board %d: %w", id, view.Board.ID, collage.ErrNotFound)
		}
		if err != nil {
			return err
		}
		f := templateFormOf(t)
		view.EditTemplate = &f
	}
	return nil
}

// templateSettings saves or deletes a template.
func (h *handlers) templateSettings(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	if v.Value("op") == "template_delete" {
		id, ok := formInt64(v, "template_id")
		if !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		err := h.store.DeleteTemplate(ctx, bc.Board.ID, id)
		if errors.Is(err, store.ErrNotFound) {
			return collage.NoContent(http.StatusNotFound), nil
		}
		if err != nil {
			return nil, err
		}
		return h.settingsDone(rc, bc, "templates", flash.Success, i18n.T(rc, "settings.saved"))
	}
	return h.saveTemplate(ctx, rc, v, bc)
}

// saveTemplate creates the template the form describes, or updates the one
// template_id names; a refused form is shown again with what was typed.
func (h *handlers) saveTemplate(ctx context.Context, rc *collage.RenderContext, v *validate.Validator, bc boardContext) (*collage.ActionResult, error) {
	posted := rc.Request.PostForm
	ids := func(name string) ([]int64, bool) {
		var out []int64
		for _, raw := range posted[name] {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return nil, false
			}
			out = append(out, id)
		}
		return out, true
	}
	form := templateForm{
		ID: v.Value("template_id"), Name: strings.TrimSpace(v.Value("template_name")),
		Title: strings.TrimSpace(v.Value("template_title")), Description: v.Value("template_description"),
		Priority: v.Value("template_priority"), Assignee: v.Value("template_assignee"), Column: v.Value("template_column"),
		DueInDays: strings.TrimSpace(v.Value("due_in_days")), Checklist: v.Value("checklist"),
		Kind: v.Value("schedule_kind"), MonthDay: strings.TrimSpace(v.Value("monthday")), Time: v.Value("schedule_time"),
	}
	var id int64
	if form.ID != "" {
		var ok bool
		if id, ok = formInt64(v, "template_id"); !ok {
			return collage.NoContent(http.StatusBadRequest), nil
		}
	}
	labels, ok := ids("template_label")
	if !ok {
		return collage.NoContent(http.StatusBadRequest), nil
	}
	form.Labels = labels
	for _, raw := range posted["weekday"] {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 6 {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		form.Weekdays |= 1 << n
	}
	column, ok := formInt64(v, "template_column")
	if !ok || !slices.Contains(scheduleKinds, form.Kind) {
		return collage.NoContent(http.StatusBadRequest), nil // a select sends one of its options
	}

	in := store.TemplateInput{Name: form.Name, Title: form.Title, Description: form.Description, ColumnID: &column, LabelIDs: labels}
	v.Field("template_name").Required().MaxLen(100)
	v.Field("template_title").Required().MaxLen(200)
	v.Field("template_description").MaxLen(10000)
	v.Field("template_priority").OneOf("1", "2", "3", "4")
	if p, err := strconv.Atoi(form.Priority); err == nil {
		priority := int16(p)
		in.Priority = &priority
	}
	if form.Assignee != "" {
		assignee, err := strconv.ParseInt(form.Assignee, 10, 64)
		if err != nil {
			return collage.NoContent(http.StatusBadRequest), nil
		}
		members, err := h.store.Members(ctx, bc.Team.ID)
		if err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(members, func(m store.Member) bool { return m.User.ID == assignee }) {
			v.Fail("template_assignee", i18n.T(rc, "rules.assignee_not_member"))
		}
		in.AssigneeID = &assignee
	}
	v.Field("due_in_days").Range(0, 365).Message(i18n.T(rc, "settings.template_invalid_due"))
	if d, err := strconv.Atoi(form.DueInDays); err == nil {
		in.DueInDays = &d
	}
	for line := range strings.SplitSeq(form.Checklist, "\n") {
		if item := strings.TrimSpace(line); item != "" {
			in.Checklist = append(in.Checklist, item)
		}
	}
	if len(in.Checklist) > maxChecklistItems ||
		slices.ContainsFunc(in.Checklist, func(s string) bool { return utf8.RuneCountInString(s) > maxChecklistText }) {
		v.Fail("checklist", i18n.T(rc, "settings.template_checklist_invalid"))
	}

	// Only the fields of the chosen kind are part of a schedule: the others,
	// sent by the form all the same, would restart it when they change.
	sc := store.Schedule{Kind: form.Kind, MonthDay: 1, Hour: 9}
	if form.Kind != "" {
		v.Field("schedule_time").Required().Matches(timeOfDay).Message(i18n.T(rc, "settings.template_invalid_time"))
	}
	if timeOfDay.MatchString(form.Time) {
		sc.Hour, _ = strconv.Atoi(form.Time[:2])
		sc.Minute, _ = strconv.Atoi(form.Time[3:])
	}
	switch form.Kind {
	case "weekly":
		if form.Weekdays == 0 {
			v.Fail("weekday", i18n.T(rc, "settings.template_no_weekday"))
		}
		sc.Weekdays = form.Weekdays
	case "monthly":
		v.Field("monthday").Required().Range(1, 31).Message(i18n.T(rc, "settings.template_invalid_monthday"))
		sc.MonthDay, _ = strconv.Atoi(form.MonthDay)
	}
	in.Schedule = sc

	refuse := func() (*collage.ActionResult, error) {
		form.Errors = v.Errors()
		rc.Set(templateDraftKey, form)
		return validate.Refuse(rc, v, rc.Page), nil
	}
	if !v.Valid() {
		return refuse()
	}
	var err error
	if id == 0 {
		_, err = h.store.CreateTemplate(ctx, bc.Board.ID, in, bc.User.ID)
	} else {
		_, err = h.store.UpdateTemplate(ctx, bc.Board.ID, id, in, bc.User.ID)
	}
	switch {
	case errors.Is(err, store.ErrTemplateName):
		v.Fail("template_name", i18n.T(rc, "settings.template_name_exists"))
		return refuse()
	case errors.Is(err, store.ErrNotFound):
		return collage.NoContent(http.StatusNotFound), nil
	case err != nil:
		return nil, err
	}
	return h.settingsDone(rc, bc, "templates", flash.Success, i18n.T(rc, "settings.saved"))
}
