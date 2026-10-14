package configui

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func keyEnter(model *teatest.TestModel) {
	model.Send(tea.KeyMsg{Type: tea.KeyEnter})
}

func keyDown(model *teatest.TestModel) {
	model.Send(tea.KeyMsg{Type: tea.KeyDown})
}

func keyQ(model *teatest.TestModel) {
	model.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("q"),
	})
}

func keyRune(model *teatest.TestModel, r rune) {
	model.Send(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{r},
	})
}

func waitForContains(t *testing.T, output io.Reader, text string) {
	t.Helper()
	teatest.WaitFor(
		t, output,
		func(bts []byte) bool {
			return strings.Contains(string(bts), text)
		},
		teatest.WithCheckInterval(20*time.Millisecond),
		teatest.WithDuration(10*time.Second),
	)
}

func waitForContainsAll(t *testing.T, output io.Reader, texts ...string) {
	t.Helper()
	teatest.WaitFor(
		t, output,
		func(bts []byte) bool {
			s := string(bts)
			for _, text := range texts {
				if !strings.Contains(s, text) {
					return false
				}
			}
			return true
		},
		teatest.WithCheckInterval(20*time.Millisecond),
		teatest.WithDuration(10*time.Second),
	)
}
