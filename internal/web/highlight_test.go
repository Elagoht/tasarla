package web

import "testing"

func TestHighlightFoldsLikeTheDatabase(t *testing.T) {
	h := highlight("Bütçe IŞIĞINDA karar", "ışığında", 0)
	if h.Before != "Bütçe " || h.Match != "IŞIĞINDA" || h.After != " karar" || h.CutStart || h.CutEnd {
		t.Errorf("got %+v", h)
	}
}

func TestHighlightKeepsAWindow(t *testing.T) {
	long := ""
	for range 100 {
		long += "a"
	}
	h := highlight(long+"HEDEF"+long, "hedef", 40)
	if h.Match != "HEDEF" || !h.CutStart || !h.CutEnd || len([]rune(h.Before+h.Match+h.After)) != 40 {
		t.Errorf("got %+v", h)
	}
	if h := highlight("kısa metin", "yok", 160); h.Before != "kısa metin" || h.Match != "" {
		t.Errorf("no match: %+v", h)
	}
}
