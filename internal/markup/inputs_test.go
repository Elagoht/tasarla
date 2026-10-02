package markup

import "testing"

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
