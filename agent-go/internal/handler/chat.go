package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/message"
)

// ListMessages handles GET /threads/{id}/messages — returns all messages for the session.
func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")
	leafID := r.URL.Query().Get("leafId")

	msgs, err := h.completions.MessagesJSON(threadID, leafID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, api.GetMessagesResponse{
		Messages: msgs,
	})
}

// PostChat handles POST /threads/{id}/chat — starts a completion and streams the response via SSE.
func (h *Handler) PostChat(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")

	var req api.ChatRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Reset hook state for user-initiated completions.
	h.resetHookState()

	// Check for active completion.
	if activeID := h.completions.ActiveCompletionID(threadID); activeID != "" {
		h.JSON(w, http.StatusConflict, api.ChatConflictResponse{
			Error:        "completion_in_progress",
			CompletionID: activeID,
		})
		return
	}

	promptReq := agent.PromptRequest{
		Model:     req.Model,
		Reasoning: req.Reasoning,
		UserParts: []message.Part{
			message.TextPart{Text: string(req.Messages)},
		},
	}

	completionID, err := h.completions.Chat(threadID, promptReq)
	if err != nil {
		if strings.Contains(err.Error(), "completion_in_progress") {
			parts := strings.SplitN(err.Error(), ":", 2)
			existingID := ""
			if len(parts) == 2 {
				existingID = parts[1]
			}
			h.JSON(w, http.StatusConflict, api.ChatConflictResponse{
				Error:        "completion_in_progress",
				CompletionID: existingID,
			})
			return
		}
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusAccepted, api.ChatStartedResponse{
		CompletionID: completionID,
		Status:       "started",
	})
}

// ChatStream handles GET /threads/{id}/chat/stream — streams SSE events for an active completion.
// Returns 204 No Content if no completion is currently running.
func (h *Handler) ChatStream(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			offset = v
		}
	}

	result := h.completions.PollChunks(threadID, offset)
	if result == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	for _, chunk := range result.Chunks {
		data, err := message.MarshalChunk(chunk)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	if result.Done {
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

// CancelChat handles POST /threads/{id}/chat/cancel — cancels in-progress completion.
func (h *Handler) CancelChat(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")

	completionID, ok := h.completions.Cancel(threadID)
	if !ok {
		h.JSON(w, http.StatusConflict, api.NoActiveCompletionResponse{
			Error: "no_active_completion",
		})
		return
	}

	h.JSON(w, http.StatusOK, api.CancelCompletionResponse{
		Success:      true,
		CompletionID: completionID,
		Status:       "cancelled",
	})
}

// GetQuestion handles GET /threads/{id}/chat/question/{questionId} — returns a pending user question.
func (h *Handler) GetQuestion(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")
	questionID := chi.URLParam(r, "questionId")

	pending, err := h.completions.PendingQuestion(threadID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if pending != nil && pending.ToolCallID == questionID {
		// Question is pending — parse and return it.
		var questions []api.AskUserQuestion
		if err := json.Unmarshal(pending.Questions, &questions); err != nil {
			h.Error(w, http.StatusInternalServerError, "failed to parse questions: "+err.Error())
			return
		}

		h.JSON(w, http.StatusOK, api.PendingQuestionResponse{
			Status: "pending",
			Question: &api.PendingQuestion{
				ToolUseID: pending.ToolCallID,
				Questions: questions,
			},
		})
		return
	}

	// No pending question — check if it was already answered.
	h.answeredMu.Lock()
	wasAnswered := h.answeredQuestions[questionID]
	h.answeredMu.Unlock()

	status := "expired"
	if wasAnswered {
		status = "answered"
	}

	h.JSON(w, http.StatusOK, api.PendingQuestionResponse{
		Status:   status,
		Question: nil,
	})
}

// PostAnswer handles POST /threads/{id}/chat/answer/{questionId} — submits answers to a pending question.
func (h *Handler) PostAnswer(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")
	questionID := chi.URLParam(r, "questionId")

	var req api.AnswerQuestionRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Answers == nil {
		h.Error(w, http.StatusBadRequest, "answers is required")
		return
	}

	// Persist the answer.
	if err := h.completions.SubmitAnswer(threadID, questionID, req.Answers); err != nil {
		h.Error(w, http.StatusNotFound, err.Error())
		return
	}

	// Track as answered for status polling.
	h.answeredMu.Lock()
	h.answeredQuestions[questionID] = true
	h.answeredMu.Unlock()

	// Resume the turn. Prompt() will detect the waiting_for_answer state
	// with the answer file on disk and continue the turn.
	completionID, chatErr := h.completions.Chat(threadID, agent.PromptRequest{})
	if chatErr != nil {
		// Answer was saved but resume failed — log but still return success.
		log.Printf("question: answer saved but failed to resume turn: %v", chatErr)
	}
	_ = completionID

	h.JSON(w, http.StatusOK, api.AnswerQuestionResponse{Success: true})
}
