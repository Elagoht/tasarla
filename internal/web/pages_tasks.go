package web

import (
	"context"
	"time"

	i18n "github.com/Elagoht/collage-i18n"
	"github.com/Elagoht/collage/pkg/collage"

	"kanban/internal/store"
)

type tasksView struct {
	Groups []taskGroup
}

type taskGroup struct {
	BoardID   int64
	BoardName string
	TeamName  string
	Tasks     []taskView
}

type taskView struct {
	Task    store.Task
	Due     string
	Overdue bool
}

func (h *handlers) tasksPage() *collage.Page {
	content := collage.NewFragment("tasks-content", "pages/tasks.html").
		WithData(collage.Load(h.loadTasks)).
		Required().
		Build()
	return paths(h.privatePage("tasks", content), "/me/tasks").Dynamic().Build()
}

func (h *handlers) loadTasks(ctx context.Context, rc *collage.RenderContext) (tasksView, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return tasksView{}, err
	}
	rc.HoistTitle(i18n.T(rc, "tasks.title"))
	tasks, err := h.store.AssignedTo(ctx, user.ID)
	if err != nil {
		return tasksView{}, err
	}
	today := time.Now().In(h.loc).Format(time.DateOnly)
	var view tasksView
	for _, t := range tasks {
		if n := len(view.Groups); n == 0 || view.Groups[n-1].BoardID != t.Card.BoardID {
			view.Groups = append(view.Groups, taskGroup{BoardID: t.Card.BoardID, BoardName: t.BoardName, TeamName: t.TeamName})
		}
		tv := taskView{Task: t}
		if t.Card.DueDate != nil {
			tv.Due = t.Card.DueDate.Format(time.DateOnly)
			tv.Overdue = tv.Due < today
		}
		g := &view.Groups[len(view.Groups)-1]
		g.Tasks = append(g.Tasks, tv)
	}
	return view, nil
}
