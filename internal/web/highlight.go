package web

import (
	"slices"
	"unicode"
)

type highlighted struct {
	Before, Match, After string
	CutStart, CutEnd     bool
}

// fold is kanban_fold (migration 008) rune by rune, keeping where each folded
// rune came from, so a match in the folded text is a span of the original.
func fold(s []rune) ([]rune, []int) {
	out, from := make([]rune, 0, len(s)), make([]int, 0, len(s))
	for i, r := range s {
		switch r {
		case '\u0307': // the dot lowering İ may leave
			continue
		case 'İ', 'I', 'ı':
			r = 'i'
		default:
			r = unicode.ToLower(r)
		}
		out, from = append(out, r), append(from, i)
	}
	return out, from
}

func highlight(text, q string, width int) highlighted {
	runes := []rune(text)
	folded, from := fold(runes)
	needle, _ := fold([]rune(q))
	start, end := -1, -1
	if len(needle) > 0 {
		for i := 0; i+len(needle) <= len(folded); i++ {
			if slices.Equal(folded[i:i+len(needle)], needle) {
				start, end = from[i], from[i+len(needle)-1]+1
				break
			}
		}
	}
	lo, hi := 0, len(runes)
	if width > 0 && len(runes) > width {
		if start < 0 {
			hi = width
		} else {
			lo = max(0, start-(width-(end-start))/3)
			hi = min(len(runes), lo+width)
			lo = max(0, hi-width)
		}
	}
	h := highlighted{CutStart: lo > 0, CutEnd: hi < len(runes)}
	if start < 0 {
		h.Before = string(runes[lo:hi])
		return h
	}
	h.Before, h.Match, h.After = string(runes[lo:max(lo, start)]), string(runes[max(lo, start):min(hi, end)]), string(runes[min(hi, end):hi])
	return h
}
