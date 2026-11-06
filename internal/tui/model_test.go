package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gleison/kraken/internal/domain"
	"github.com/gleison/kraken/internal/llm"
	"github.com/gleison/kraken/internal/orchestrator"
)

type tuiClientFunc func(context.Context, llm.Request) (*llm.Response, error)

func (f tuiClientFunc) Complete(ctx context.Context, request llm.Request) (*llm.Response, error) {
	return f(ctx, request)
}

func TestCtrlCCancelsActiveRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	model := Model{phase: phaseRunning, cancel: cancel}

	updated, cmd := model.handleKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
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

	_, cmd := model.handleKey(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("escape should not quit")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("active context was not canceled")
	}
}

func TestModelStartsInChatUnlessPlanModeIsRequested(t *testing.T) {
	if model := NewModel(nil); model.mode != modeChat {
		t.Fatalf("default mode = %v", model.mode)
	}
	if model := NewModel(nil, WithPlanMode()); model.mode != modePlan {
		t.Fatalf("configured mode = %v", model.mode)
	}
}

func TestStartRunRoutesToSelectedMode(t *testing.T) {
	for _, test := range []struct {
		name       string
		options    []ModelOption
		wantSchema bool
	}{
		{name: "chat by default", wantSchema: false},
		{name: "plan when configured", options: []ModelOption{WithPlanMode()}, wantSchema: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := make(chan llm.Request, 3)
			client := tuiClientFunc(func(_ context.Context, request llm.Request) (*llm.Response, error) {
				requests <- request
				if request.JSONSchema != nil {
					return &llm.Response{Content: `{"tasks":[{"title":"one","instruction":"first"},{"title":"two","instruction":"output"}]}`}, nil
				}
				return &llm.Response{Content: "done"}, nil
			})
			model := NewModel(orchestrator.New(client), test.options...)
			model.startRun("work")

			firstRequest := <-requests
			if got := firstRequest.JSONSchema != nil; got != test.wantSchema {
				t.Fatalf("structured planning request = %t, want %t", got, test.wantSchema)
			}
			for range model.events {
			}
			model.finishRun()
		})
	}
}

func TestModeCommandsSwitchWithoutRunning(t *testing.T) {
	model := NewModel(nil)
	model.input.value = []rune("/plan")

	updated, cmd := model.submitInput()
	planModel := updated.(Model)
	if cmd != nil || planModel.mode != modePlan || planModel.input.Value() != "" {
		t.Fatalf("plan command produced %#v, cmd=%v", planModel, cmd)
	}

	planModel.input.value = []rune("/chat")
	updated, cmd = planModel.submitInput()
	chatModel := updated.(Model)
	if cmd != nil || chatModel.mode != modeChat || chatModel.input.Value() != "" {
		t.Fatalf("chat command produced %#v, cmd=%v", chatModel, cmd)
	}
}

func TestPlanCommandCanRunAnInlineGoal(t *testing.T) {
	client := tuiClientFunc(func(_ context.Context, request llm.Request) (*llm.Response, error) {
		if request.JSONSchema != nil {
			return &llm.Response{Content: `{"tasks":[{"title":"one","instruction":"first"},{"title":"two","instruction":"output"}]}`}, nil
		}
		return &llm.Response{Content: "done"}, nil
	})
	model := NewModel(orchestrator.New(client))
	model.input.value = []rune("/plan do the work")

	updated, cmd := model.submitInput()
	running := updated.(Model)
	if cmd == nil || running.mode != modePlan || running.phase != phaseRunning {
		t.Fatalf("model = %#v, cmd=%v", running, cmd)
	}
	if running.pendingInput != "do the work" || running.goal != "do the work" {
		t.Fatalf("input=%q goal=%q", running.pendingInput, running.goal)
	}
	running.finishRun()
}

func TestChatCompletionIsStoredWithoutPlan(t *testing.T) {
	events := make(chan orchestrator.Event)
	model := Model{
		phase:        phaseRunning,
		mode:         modeChat,
		input:        newTextInput(""),
		pendingInput: "hello",
		goal:         "hello",
		events:       events,
	}
	model.input.value = []rune("next message")

	updated, _ := model.handleEvent(eventMsg{ok: true, event: orchestrator.Event{
		Type:  orchestrator.EventRunCompleted,
		Final: "hi",
	}})
	completed := updated.(Model)
	if completed.phase != phaseInput || len(completed.session) != 1 {
		t.Fatalf("completed model = %#v", completed)
	}
	turn := completed.session[0]
	if turn.Plan != nil || turn.UserInput != "hello" || turn.Result != "hi" {
		t.Fatalf("turn = %#v", turn)
	}
	if completed.input.Value() != "next message" {
		t.Fatalf("draft was lost: %q", completed.input.Value())
	}
	view := completed.View().Content
	if !strings.Contains(view, "hi") || !strings.Contains(view, "Mensagem") {
		t.Fatalf("chat view does not contain response and composer: %q", view)
	}
}

func TestCtrlEnterSubmits(t *testing.T) {
	if !isSubmitKey(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl}) {
		t.Fatal("ctrl+enter was not recognized")
	}
	if isSubmitKey(tea.KeyPressMsg{Code: tea.KeyEnter}) {
		t.Fatal("plain enter must remain a newline")
	}
}

func TestChatAcceptsNextDraftWhileResponseIsRunning(t *testing.T) {
	model := Model{phase: phaseRunning, mode: modeChat, input: newTextInput("")}
	updated, cmd := model.handleKey(tea.KeyPressMsg{Code: 'n', Text: "next"})
	if cmd != nil {
		t.Fatal("typing a draft should not start another request")
	}
	if got := updated.(Model).input.Value(); got != "next" {
		t.Fatalf("draft = %q", got)
	}
}

func TestUserInputUsesChatResponseAlignment(t *testing.T) {
	model := Model{width: 80}
	if got := model.renderUserInput("hello\nworld"); got != "  hello\n  world" {
		t.Fatalf("rendered input = %q", got)
	}
	if got := model.renderUserMessage("hello"); !strings.Contains(got, "você\n\n  hello") {
		t.Fatalf("rendered message = %q", got)
	}
}

func TestLatestChatTurnKeepsUserHeaderVisible(t *testing.T) {
	model := Model{
		mode: modeChat,
		session: []domain.Turn{
			{UserInput: "first", Result: "answer one"},
			{UserInput: "second", Result: "answer two"},
		},
	}
	offset := model.latestChatTurnOffset()
	lines := strings.Split(model.renderHistory(), "\n")
	if offset >= len(lines) || !strings.Contains(lines[offset], "você") {
		t.Fatalf("offset %d does not start at user header: %#v", offset, lines)
	}
}
