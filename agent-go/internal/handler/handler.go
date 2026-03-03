package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/hooks"
	"github.com/obot-platform/discobot/agent-go/internal/services"
	"github.com/obot-platform/discobot/agent-go/message"
)

const maxHookRetries = 3

// Handler contains all HTTP handlers for the agent API.
type Handler struct {
	agentCwd       string
	completions    *agent.CompletionManager
	hookManager    *hooks.Manager    // nil if hooks are disabled
	serviceManager *services.Manager // always initialized

	hookMu         sync.Mutex
	hookRetryCount int

	answeredMu        sync.Mutex
	answeredQuestions map[string]bool // toolCallID → true (tracks answered questions for status polling)
}

// New creates a new Handler.
func New(agentCwd string, completions *agent.CompletionManager, hookManager *hooks.Manager, serviceManager *services.Manager) *Handler {
	h := &Handler{
		agentCwd:          agentCwd,
		completions:       completions,
		hookManager:       hookManager,
		serviceManager:    serviceManager,
		answeredQuestions: make(map[string]bool),
	}

	// Wire up post-completion hook evaluation.
	if hookManager != nil && hookManager.HasFileHooks() {
		completions.SetOnTurnComplete(h.onTurnComplete)
	}

	return h
}

// onTurnComplete is called when a completion finishes. It schedules hook evaluation.
func (h *Handler) onTurnComplete(threadID string, _ error) {
	if h.hookManager == nil || !h.hookManager.HasFileHooks() {
		return
	}
	// Fire-and-forget goroutine matching the TS scheduleHookEvaluation pattern.
	go h.scheduleHookEvaluation(threadID)
}

// scheduleHookEvaluation runs hook evaluation after a grace period, and
// triggers a re-prompt if a hook fails with notifyLlm=true.
func (h *Handler) scheduleHookEvaluation(threadID string) {
	// 200ms grace period to let SSE flush
	time.Sleep(200 * time.Millisecond)

	result := h.hookManager.EvaluateFileHooks()
	if !result.ShouldReprompt {
		return
	}

	h.hookMu.Lock()
	h.hookRetryCount++
	count := h.hookRetryCount
	h.hookMu.Unlock()

	if count >= maxHookRetries {
		log.Printf("hooks: max retries (%d) reached, not re-prompting", maxHookRetries)
		return
	}

	req := agent.PromptRequest{
		UserParts: []message.Part{
			message.TextPart{Text: result.LLMMessage},
		},
	}

	if _, err := h.completions.Chat(threadID, req); err != nil {
		log.Printf("hooks: failed to start re-prompt: %v", err)
	}
}

// resetHookState aborts any pending hook evaluation and resets the retry counter.
func (h *Handler) resetHookState() {
	h.hookMu.Lock()
	h.hookRetryCount = 0
	h.hookMu.Unlock()
}

// RegisterRoutes registers all API routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Global (non-session) routes
	r.Get("/", h.Root)
	r.Get("/health", h.Health)
	r.Get("/user", h.User)

	// Thread routes (a session can have multiple threads)
	r.Get("/threads", h.ListThreads)
	r.Route("/threads/{id}", func(r chi.Router) {
		r.Get("/models", h.ListModels)
		r.Get("/messages", h.ListMessages)

		r.Post("/chat", h.PostChat)
		r.Get("/chat/stream", h.ChatStream)
		r.Post("/chat/cancel", h.CancelChat)
		r.Get("/chat/question/{questionId}", h.GetQuestion)
		r.Post("/chat/answer/{questionId}", h.PostAnswer)
	})

	// File system routes
	r.Get("/files", h.ListFiles)
	r.Get("/files/search", h.SearchFiles)
	r.Get("/files/read", h.ReadFile)
	r.Post("/files/write", h.WriteFile)
	r.Post("/files/delete", h.DeleteFile)
	r.Post("/files/rename", h.RenameFile)

	// Diff route
	r.Get("/diff", h.GetDiff)

	// Git commits route
	r.Get("/commits", h.GetCommits)

	// Hook routes
	r.Get("/hooks/status", h.HooksStatus)
	r.Get("/hooks/{hookId}/output", h.HookOutput)
	r.Post("/hooks/{hookId}/rerun", h.RerunHook)

	// Service routes
	r.Get("/services", h.ListServices)
	r.Post("/services/{serviceId}/start", h.StartService)
	r.Post("/services/{serviceId}/stop", h.StopService)
	r.Get("/services/{serviceId}/output", h.ServiceOutput)
	r.HandleFunc("/services/{serviceId}/http/*", h.ServiceProxy)
}

// JSON writes a JSON response with the given status code.
func (h *Handler) JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// Error writes a JSON error response.
func (h *Handler) Error(w http.ResponseWriter, status int, message string) {
	h.JSON(w, status, map[string]string{"error": message})
}

// DecodeJSON decodes the request body as JSON.
func (h *Handler) DecodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
