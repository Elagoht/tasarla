package markup

import (
	"strings"
	"testing"
)

func TestOnlyTaskCheckboxes(t *testing.T) {
	cases := map[string]string{
		`<input checked="" disabled="" type="checkbox">`: `<input checked="" disabled="" type="checkbox">`,
		`<input disabled="" type="checkbox">`:            `<input disabled="" type="checkbox">`,
		`<input type="checkbox">`:                        ``,
		`<input disabled="">`:                            ``,
		`<input>`:                                        ``,
		`a <input type="checkbox"> b`:                    `a  b`,
	}
	for in, want := range cases {
		if got := onlyTaskCheckboxes(in); got != want {
			t.Errorf("onlyTaskCheckboxes(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanDropsOtherInputs(t *testing.T) {
	for _, in := range []string{`<input type="checkbox">`, `<input disabled>`, `<input type="text" disabled="">`} {
		if out := clean([]byte(in)); strings.Contains(out, "<input") {
			t.Errorf("clean(%q) = %q, kept an input", in, out)
		}
	}
	if out := clean([]byte(`<input checked="" disabled="" type="checkbox">`)); !strings.Contains(out, "<input") {
		t.Errorf("clean dropped a task checkbox: %q", out)
	}
}

func TestMarkdownKeepsOneDisabledCheckbox(t *testing.T) {
	out := string(Markdown("- [x] a"))
	if strings.Count(out, "<input") != 1 || !strings.Contains(out, `disabled=""`) || !strings.Contains(out, `type="checkbox"`) {
		t.Errorf("Markdown(task) = %q", out)
	}
}
