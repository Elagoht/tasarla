package web

// boardTabsView feeds the "board-tabs" partial: the board's views as tabs,
// the one being shown marked.
type boardTabsView struct {
	BoardID int64
	Active  string
}

// boardTabs is the template func behind the partial, since a template call
// takes a single value.
func boardTabs(boardID int64, active string) boardTabsView {
	return boardTabsView{BoardID: boardID, Active: active}
}
