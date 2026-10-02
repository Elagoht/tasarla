package web

import "net/http"

// boardTabsView feeds the "board-tabs" partial: the board's views as tabs,
// the one being shown marked.
type boardTabsView struct {
	BoardID int64
	Active  string
}

// boardWideCookie holds a viewer's choice to let the board take the whole
// width (board-width.js sets it); read on the server, so the page is drawn at
// that width from the start.
const boardWideCookie = "board_wide"

func boardWide(r *http.Request) bool {
	c, err := r.Cookie(boardWideCookie)
	return err == nil && c.Value == "1"
}

// boardTabs is the template func behind the partial, since a template call
// takes a single value.
func boardTabs(boardID int64, active string) boardTabsView {
	return boardTabsView{BoardID: boardID, Active: active}
}
