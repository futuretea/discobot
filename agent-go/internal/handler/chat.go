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

	// Check for active completion.
	if activeID := h.completions.ActiveCompletionID(threadID); activeID != "" {
		h.JSON(w, http.StatusConflict, api.ChatConflictResponse{
			Error:        "completion_in_progress",
			CompletionID: activeID,
		})
		return
	}

	// Reset hook state for user-initiated completions.
	h.resetHookState()

	promptReq := agent.PromptRequest{
		Model:     req.Model,
		Reasoning: req.Reasoning,
		UserParts: extractUserParts(req.Messages),
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

// ChatStream handles GET /threads/{id}/chat/stream — streams SSE events for a completion.
// Returns 204 No Content only if no completion record exists at all.
// If a completion exists (active or finished), responds with 200 and sets the
// X-Discobot-Completion-Active header to "true" or "false" before any SSE data is written,
// so callers can determine activity state from the initial response headers.
//
// Each SSE event carries an id of the form "{completionID}:{offset}" so that
// clients can resume after a dropped connection by sending Last-Event-ID.
// If the completion ID in Last-Event-ID does not match the current completion,
// the stream restarts from offset 0 (a new completion is in progress).
func (h *Handler) ChatStream(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")

	// Return 204 only if there is no completion record at all.
	snapshot := h.completions.PollChunks(threadID, 0)
	if snapshot == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Determine starting offset: Last-Event-ID takes precedence (reconnection),
	// then the offset query param, then 0.
	offset := 0
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil {
		offset = v
	}
	if lastEventID := r.Header.Get("Last-Event-ID"); lastEventID != "" {
		if id, n, ok := parseSSEEventID(lastEventID); ok {
			if id == snapshot.CompletionID {
				// Same completion — resume after the last received chunk.
				offset = n + 1
			}
			// Different completion ID: a new completion is running; stream from
			// the beginning (or the query-param offset, already set above).
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Set X-Discobot-Completion-Active before WriteHeader so callers can read it from
	// the initial response before any SSE data arrives.
	isActive := !snapshot.Done
	w.Header().Set("X-Discobot-Completion-Active", strconv.FormatBool(isActive))
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		result := h.completions.WaitChunks(r.Context(), threadID, offset)
		if result == nil {
			// Completion was cleaned up — signal done to the client.
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		for _, chunk := range result.Chunks {
			data, err := message.MarshalChunk(chunk)
			if err != nil {
				offset++
				continue
			}
			fmt.Fprintf(w, "id: %s:%d\n", result.CompletionID, offset)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			offset++
		}

		if result.Done {
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		if r.Context().Err() != nil {
			return
		}
	}
}

// parseSSEEventID parses a "{completionID}:{offset}" SSE event ID.
func parseSSEEventID(id string) (completionID string, offset int, ok bool) {
	idx := strings.LastIndex(id, ":")
	if idx < 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(id[idx+1:])
	if err != nil {
		return "", 0, false
	}
	return id[:idx], n, true
}

// ChatStatus handles GET /threads/{id}/chat/status — returns whether a completion is active.
// Unlike ChatStream, this never opens an SSE connection; it is safe to poll frequently.
func (h *Handler) ChatStatus(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "id")
	isRunning := h.completions.ActiveCompletionID(threadID) != ""
	h.JSON(w, http.StatusOK, api.ChatStatusResponse{IsRunning: isRunning})
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

// extractUserParts parses the last user message from the AI SDK UIMessages JSON array
// and returns its parts as message.Part values. This converts the frontend wire format
// to the agent's internal representation using UnmarshalUIPart for type dispatch.
func extractUserParts(msgs json.RawMessage) []message.Part {
	if len(msgs) == 0 {
		return nil
	}
	var rawMsgs []struct {
		Role  string            `json:"role"`
		Parts []json.RawMessage `json:"parts"`
	}
	if err := json.Unmarshal(msgs, &rawMsgs); err != nil {
		// Fallback: treat the whole blob as plain text (CLI compat).
		return []message.Part{message.TextPart{Text: string(msgs)}}
	}
	for i := len(rawMsgs) - 1; i >= 0; i-- {
		if rawMsgs[i].Role != "user" {
			continue
		}
		var parts []message.Part
		for _, partData := range rawMsgs[i].Parts {
			p, err := message.UnmarshalUIPart(partData)
			if err != nil {
				continue
			}
			switch v := p.(type) {
			case message.UITextPart:
				parts = append(parts, message.TextPart{Text: v.Text})
			case message.UIFilePart:
				parts = append(parts, message.FilePart{Data: v.URL, MediaType: v.MediaType, Filename: v.Filename})
			}
		}
		if len(parts) > 0 {
			return parts
		}
	}
	return nil
}
