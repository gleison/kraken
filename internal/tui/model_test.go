package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCtrlCCancelsActiveRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	model := Model{phase: phaseRunning, cancel: cancel}

	updated, cmd := model.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("active context was not canceled")
	}
	if updated.(Model).cancel != nil {
		t.Fatal("cancel function was not cleared")
	}
}

func TestEscapeCancelsWithoutQuitting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	model := Model{phase: phaseRunning, cancel: cancel}

	_, cmd := model.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("escape should not quit")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("active context was not canceled")
	}
}
