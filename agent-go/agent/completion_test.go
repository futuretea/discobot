package agent

import (
	"context"
	"encoding/json"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// --- Mock agent for completion tests ---

type mockAgent struct {
	promptFn func(ctx context.Context, threadID string, req PromptRequest) iter.Seq2[message.MessageChunk, error]

	interruptedThreads []string
	models             []providers.ModelInfo
	threads            []string
}

func (m *mockAgent) Prompt(ctx context.Context, threadID string, req PromptRequest) iter.Seq2[message.MessageChunk, error] {
	if m.promptFn != nil {
		return m.promptFn(ctx, threadID, req)
	}
	return func(_ func(message.MessageChunk, error) bool) {}
}

func (m *mockAgent) Cancel(_ string) bool {
	return false
}

func (m *mockAgent) Messages(_, _ string) ([]json.RawMessage, error) {
	return nil, nil
}

func (m *mockAgent) ListModels(_ context.Context) ([]providers.ModelInfo, error) {
	return m.models, nil
}

func (m *mockAgent) ListThreads() ([]string, error) {
	return m.threads, nil
}

func (m *mockAgent) InterruptedThreads() ([]string, error) {
	return m.interruptedThreads, nil
}

func (m *mockAgent) PendingQuestion(_ string) (*thread.PendingQuestionState, error) {
	return nil, nil
}

func (m *mockAgent) SubmitAnswer(_, _ string, _ map[string]string) error {
	return nil
}

func (m *mockAgent) FinalResponse(_ string) (string, error) {
	return "", nil
}

// --- Helpers ---

func simplePromptFn(chunks []message.MessageChunk) func(context.Context, string, PromptRequest) iter.Seq2[message.MessageChunk, error] {
	return func(_ context.Context, _ string, _ PromptRequest) iter.Seq2[message.MessageChunk, error] {
		return func(yield func(message.MessageChunk, error) bool) {
			for _, c := range chunks {
				if !yield(c, nil) {
					return
				}
			}
		}
	}
}

func blockingPromptFn() func(context.Context, string, PromptRequest) iter.Seq2[message.MessageChunk, error] {
	return func(ctx context.Context, _ string, _ PromptRequest) iter.Seq2[message.MessageChunk, error] {
		return func(_ func(message.MessageChunk, error) bool) {
			<-ctx.Done()
		}
	}
}

func waitForDone(t *testing.T, cm *CompletionManager, threadID string) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for completion")
		default:
			result := cm.PollChunks(threadID, 0)
			if result != nil && result.Done {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// --- Tests ---

func TestCompletionManager_Chat_SimpleCompletion(t *testing.T) {
	chunks := []message.MessageChunk{
		message.StreamStartChunk{},
		message.TextStartChunk{ID: "t1"},
		message.TextDeltaChunk{ID: "t1", Delta: "Hello!"},
		message.TextEndChunk{ID: "t1"},
		message.FinishChunk{FinishReason: message.FinishReason{Unified: "stop"}},
	}

	agent := &mockAgent{promptFn: simplePromptFn(chunks)}
	cm := NewCompletionManager(agent)

	completionID, err := cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if completionID == "" {
		t.Error("expected non-empty completion ID")
	}

	waitForDone(t, cm, "thread1")

	result := cm.PollChunks("thread1", 0)
	if result == nil {
		t.Fatal("expected poll result")
	}
	if !result.Done {
		t.Error("expected completion to be done")
	}
	if len(result.Chunks) == 0 {
		t.Error("expected at least some chunks")
	}

	hasText := false
	for _, c := range result.Chunks {
		if _, ok := c.(message.TextDeltaChunk); ok {
			hasText = true
		}
	}
	if !hasText {
		t.Error("missing TextDeltaChunk")
	}
}

func TestCompletionManager_Chat_ConflictWhenActive(t *testing.T) {
	agent := &mockAgent{promptFn: blockingPromptFn()}
	cm := NewCompletionManager(agent)

	_, err := cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Second chat should fail.
	_, err = cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "hi again"}},
	})
	if err == nil {
		t.Error("expected error for concurrent completion")
	}
	if !strings.Contains(err.Error(), "completion_in_progress") {
		t.Errorf("expected completion_in_progress error, got: %v", err)
	}

	// Cancel to clean up.
	cm.Cancel("thread1")
	waitForDone(t, cm, "thread1")
}

func TestCompletionManager_Chat_DifferentThreadsIndependent(t *testing.T) {
	callCount := 0
	agent := &mockAgent{
		promptFn: func(_ context.Context, _ string, _ PromptRequest) iter.Seq2[message.MessageChunk, error] {
			return func(yield func(message.MessageChunk, error) bool) {
				callCount++
				yield(message.TextDeltaChunk{ID: "t1", Delta: "ok"}, nil)
			}
		},
	}
	cm := NewCompletionManager(agent)

	_, err1 := cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "a"}},
	})
	_, err2 := cm.Chat("thread2", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "b"}},
	})

	if err1 != nil {
		t.Fatal(err1)
	}
	if err2 != nil {
		t.Fatal(err2)
	}

	waitForDone(t, cm, "thread1")
	waitForDone(t, cm, "thread2")
}

func TestCompletionManager_Cancel(t *testing.T) {
	agent := &mockAgent{promptFn: blockingPromptFn()}
	cm := NewCompletionManager(agent)

	completionID, err := cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)

	cancelledID, ok := cm.Cancel("thread1")
	if !ok {
		t.Error("expected successful cancellation")
	}
	if cancelledID != completionID {
		t.Errorf("expected cancelled ID=%s, got %s", completionID, cancelledID)
	}

	waitForDone(t, cm, "thread1")
}

func TestCompletionManager_Cancel_NoActive(t *testing.T) {
	agent := &mockAgent{}
	cm := NewCompletionManager(agent)

	_, ok := cm.Cancel("thread1")
	if ok {
		t.Error("expected no active completion to cancel")
	}
}

func TestCompletionManager_PollChunks_NoActiveCompletion(t *testing.T) {
	agent := &mockAgent{}
	cm := NewCompletionManager(agent)

	result := cm.PollChunks("thread1", 0)
	if result != nil {
		t.Error("expected nil for non-existent thread")
	}
}

func TestCompletionManager_PollChunks_WithOffset(t *testing.T) {
	chunks := []message.MessageChunk{
		message.StreamStartChunk{},
		message.TextStartChunk{ID: "t1"},
		message.TextDeltaChunk{ID: "t1", Delta: "hello"},
		message.TextEndChunk{ID: "t1"},
		message.FinishChunk{FinishReason: message.FinishReason{Unified: "stop"}},
	}

	agent := &mockAgent{promptFn: simplePromptFn(chunks)}
	cm := NewCompletionManager(agent)

	_, err := cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	waitForDone(t, cm, "thread1")

	allResult := cm.PollChunks("thread1", 0)
	total := len(allResult.Chunks)
	if total == 0 {
		t.Fatal("expected chunks")
	}

	// Poll with offset should return fewer chunks.
	partial := cm.PollChunks("thread1", total-1)
	if len(partial.Chunks) != 1 {
		t.Errorf("expected 1 chunk from offset, got %d", len(partial.Chunks))
	}

	// Poll past all chunks should return empty.
	empty := cm.PollChunks("thread1", total)
	if len(empty.Chunks) != 0 {
		t.Errorf("expected 0 chunks past end, got %d", len(empty.Chunks))
	}
}

func TestCompletionManager_ActiveCompletionID(t *testing.T) {
	agent := &mockAgent{promptFn: blockingPromptFn()}
	cm := NewCompletionManager(agent)

	// No active completion.
	if id := cm.ActiveCompletionID("thread1"); id != "" {
		t.Errorf("expected empty, got %s", id)
	}

	completionID, err := cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)

	if id := cm.ActiveCompletionID("thread1"); id != completionID {
		t.Errorf("expected %s, got %s", completionID, id)
	}

	cm.Cancel("thread1")
	waitForDone(t, cm, "thread1")
}

func TestCompletionManager_ChatAfterDone(t *testing.T) {
	callCount := 0
	agent := &mockAgent{
		promptFn: func(_ context.Context, _ string, _ PromptRequest) iter.Seq2[message.MessageChunk, error] {
			callCount++
			return func(yield func(message.MessageChunk, error) bool) {
				yield(message.TextDeltaChunk{ID: "t1", Delta: "ok"}, nil)
			}
		},
	}
	cm := NewCompletionManager(agent)

	_, err := cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	waitForDone(t, cm, "thread1")

	// Starting a new chat on same thread should succeed after previous is done.
	_, err = cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "hi again"}},
	})
	if err != nil {
		t.Fatalf("expected chat to succeed after previous done: %v", err)
	}

	waitForDone(t, cm, "thread1")
}

func TestCompletionManager_SetOnTurnComplete(t *testing.T) {
	chunks := []message.MessageChunk{
		message.TextDeltaChunk{ID: "t1", Delta: "done"},
	}

	agent := &mockAgent{promptFn: simplePromptFn(chunks)}
	cm := NewCompletionManager(agent)

	completedCh := make(chan string, 1)
	cm.SetOnTurnComplete(func(threadID string, _ error) {
		completedCh <- threadID
	})

	_, err := cm.Chat("thread1", PromptRequest{
		UserParts: []message.Part{message.TextPart{Text: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case tid := <-completedCh:
		if tid != "thread1" {
			t.Errorf("expected thread1, got %s", tid)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for turn complete callback")
	}
}
