package wizard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"strings"
	"testing"

	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/mcver"
)

func TestVersionOptions(t *testing.T) {
	versions := []mcver.Version{
		mcver.MustParse("26.3"),
		mcver.MustParse("26.2"),
		mcver.MustParse("1.21.11"),
	}
	options := versionOptions(versions)

	if len(options) != len(versions)+1 {
		t.Fatalf("got %d options, want %d versions plus the escape hatch", len(options), len(versions))
	}

	if options[0].Value != "26.3" {
		t.Errorf("first value = %q, want 26.3", options[0].Value)
	}
	if !strings.Contains(options[0].Key, "newest") {
		t.Errorf("first label = %q, want it marked as newest", options[0].Key)
	}
	if strings.Contains(options[1].Key, "newest") {
		t.Errorf("second label = %q, only the first should be marked", options[1].Key)
	}

	last := options[len(options)-1]
	if last.Value != otherSentinel {
		t.Errorf("last value = %q, want the sentinel", last.Value)
	}
	for _, o := range options[:len(options)-1] {
		if o.Value == otherSentinel {
			t.Errorf("option %q collides with the sentinel", o.Key)
		}
	}
}

func TestVersionOptionsEmpty(t *testing.T) {

	options := versionOptions(nil)
	if len(options) != 1 || options[0].Value != otherSentinel {
		t.Errorf("got %d options, want just the escape hatch", len(options))
	}
}

func TestValidateRAM(t *testing.T) {
	valid := []string{"1", "4", "16", " 8 ", "256"}
	for _, s := range valid {
		if err := validateRAM(s); err != nil {
			t.Errorf("validateRAM(%q) = %v, want nil", s, err)
		}
	}
	invalid := []string{"", "cuatro", "0", "-4", "257", "4.5", "4GB"}
	for _, s := range invalid {
		if err := validateRAM(s); err == nil {
			t.Errorf("validateRAM(%q) accepted an unusable value", s)
		}
	}
}

func TestValidateRAMMatchesConfigBounds(t *testing.T) {

	cfg := config.Default()
	cfg.Name = "x"
	cfg.Software = config.Paper
	cfg.Dir = "."

	for _, ram := range []int{config.MinRAM, config.MaxRAM} {
		cfg.RAM = ram
		if err := cfg.Validate(); err != nil {
			t.Errorf("Config rejects RAM %d that the wizard accepts: %v", ram, err)
		}
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"mi-servidor", "servidor_1", "Survival 2026", " con-espacios "}
	for _, s := range valid {
		if err := validateName(s); err != nil {
			t.Errorf("validateName(%q) = %v, want nil", s, err)
		}
	}

	invalid := []string{"", "   ", "a/b", `a\b`, "c:nope", "what?", "wild*card", `quo"te`, "a<b", "a>b", "a|b"}
	for _, s := range invalid {
		if err := validateName(s); err == nil {
			t.Errorf("validateName(%q) accepted a name that cannot be a directory", s)
		}
	}
}

func renderForm(t *testing.T, form *huh.Form) string {
	t.Helper()
	form.Update(form.Init())
	return form.View()
}

func pressDown(t *testing.T, form *huh.Form, n int) string {
	t.Helper()
	var model tea.Model = form
	model.Update(form.Init())
	for i := 0; i < n; i++ {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	return model.View()
}

func testVersions(n int) []mcver.Version {
	all := []mcver.Version{
		mcver.MustParse("26.3"), mcver.MustParse("26.2"), mcver.MustParse("26.1.2"),
		mcver.MustParse("1.21.11"), mcver.MustParse("1.21.10"), mcver.MustParse("1.21.9"),
		mcver.MustParse("1.21.8"), mcver.MustParse("1.21.7"),
	}
	return all[:n]
}

func TestVersionListNeverScrolls(t *testing.T) {
	versions := testVersions(versionChoices)
	choice := versions[0].Raw
	form := versionForm(versions, &choice)

	view := renderForm(t, form)
	for _, v := range versions {
		if !strings.Contains(view, v.Raw) {
			t.Errorf("version %s is not on screen:\n%s", v.Raw, view)
		}
	}
	if !strings.Contains(view, "another…") {
		t.Errorf("the escape hatch is not on screen:\n%s", view)
	}
}

func TestVersionListStaysPutWhileNavigating(t *testing.T) {

	versions := testVersions(versionChoices)

	for _, presses := range []int{0, 1, 4, versionChoices} {
		choice := versions[0].Raw
		form := versionForm(versions, &choice)
		view := pressDown(t, form, presses)

		for _, v := range versions {
			if !strings.Contains(view, v.Raw) {
				t.Errorf("after %d down presses, %s disappeared:\n%s", presses, v.Raw, view)
			}
		}
		if !strings.Contains(view, "another…") {
			t.Errorf("after %d down presses, the escape hatch disappeared:\n%s", presses, view)
		}
	}
}

func TestVersionFormWithOneVersion(t *testing.T) {

	versions := testVersions(1)
	choice := versions[0].Raw
	view := renderForm(t, versionForm(versions, &choice))

	if !strings.Contains(view, "26.3") || !strings.Contains(view, "another…") {
		t.Errorf("single-version menu is missing entries:\n%s", view)
	}
}
