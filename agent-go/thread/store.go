package thread

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/obot-platform/discobot/agent-go/message"
)

// StoredMessage is a single message persisted on disk.
// Walking the ParentID chain from leaf to root (then reversing)
// gives the chronological conversation history.
type StoredMessage struct {
	ID       string          `json:"id"`
	ParentID string          `json:"parentId,omitempty"`
	Message  message.Message `json:"message"`
}

// Store handles file I/O for thread persistence.
// Each thread is a directory under baseDir containing message files and
// step JSONL files for replay/recovery.
type Store struct {
	baseDir string
}

// NewStore creates a new Store rooted at baseDir.
func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// messagesDir returns the path to the messages directory for a thread.
func (s *Store) messagesDir(threadID string) string {
	return filepath.Join(s.baseDir, threadID, "messages")
}

// turnsDir returns the path to the turns directory for a thread.
func (s *Store) turnsDir(threadID string) string {
	return filepath.Join(s.baseDir, threadID, "turns")
}

// SaveMessage persists a single StoredMessage to disk.
func (s *Store) SaveMessage(threadID string, msg StoredMessage) error {
	dir := s.messagesDir(threadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create messages dir: %w", err)
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	path := filepath.Join(dir, msg.ID+".json")
	return os.WriteFile(path, data, 0o644)
}

// LoadMessage reads a single StoredMessage from disk.
func (s *Store) LoadMessage(threadID, msgID string) (StoredMessage, error) {
	path := filepath.Join(s.messagesDir(threadID), msgID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return StoredMessage{}, fmt.Errorf("read message %s: %w", msgID, err)
	}
	var msg StoredMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return StoredMessage{}, fmt.Errorf("unmarshal message %s: %w", msgID, err)
	}
	return msg, nil
}

// BuildHistory loads the parent chain from leafID to root, then reverses
// to produce chronological conversation history.
func (s *Store) BuildHistory(threadID, leafID string) ([]message.Message, error) {
	var chain []message.Message
	currentID := leafID
	for currentID != "" {
		msg, err := s.LoadMessage(threadID, currentID)
		if err != nil {
			return nil, fmt.Errorf("build history: %w", err)
		}
		chain = append(chain, msg.Message)
		currentID = msg.ParentID
	}
	// Reverse to chronological order.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// HistoryEntry pairs a message with its stored ID and parent ID.
type HistoryEntry struct {
	ID       string
	ParentID string
	Message  message.Message
}

// BuildHistoryWithIDs is like BuildHistory but also returns message IDs.
// Used by compaction to map history indices to message IDs.
func (s *Store) BuildHistoryWithIDs(threadID, leafID string) ([]HistoryEntry, error) {
	var chain []HistoryEntry
	currentID := leafID
	for currentID != "" {
		msg, err := s.LoadMessage(threadID, currentID)
		if err != nil {
			return nil, fmt.Errorf("build history: %w", err)
		}
		chain = append(chain, HistoryEntry(msg))
		currentID = msg.ParentID
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// ListThreads returns the IDs of all threads in the store.
func (s *Store) ListThreads() ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list threads: %w", err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

// CreateStepFile creates (or truncates) a JSONL file for a given step
// within a turn. The caller is responsible for closing the file.
func (s *Store) CreateStepFile(threadID, turnID string, step int) (*os.File, error) {
	dir := filepath.Join(s.turnsDir(threadID), turnID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create turn dir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("step-%03d.jsonl", step))
	return os.Create(path)
}

// AppendChunk writes a single ProviderMessageChunk as a JSON line to the given file.
func (s *Store) AppendChunk(f *os.File, chunk message.ProviderMessageChunk) error {
	data, err := message.MarshalProviderChunk(chunk)
	if err != nil {
		return fmt.Errorf("marshal chunk: %w", err)
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

// --- Turn State Persistence ---

// turnStatePath returns the path to the turn state file for a thread.
func (s *Store) turnStatePath(threadID string) string {
	return filepath.Join(s.baseDir, threadID, "turn.json")
}

// SaveTurnState persists the active turn state to disk.
func (s *Store) SaveTurnState(threadID string, state TurnState) error {
	dir := filepath.Join(s.baseDir, threadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create thread dir: %w", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal turn state: %w", err)
	}
	return os.WriteFile(s.turnStatePath(threadID), data, 0o644)
}

// LoadTurnState loads the active turn state from disk.
// Returns nil if no active turn exists.
func (s *Store) LoadTurnState(threadID string) (*TurnState, error) {
	data, err := os.ReadFile(s.turnStatePath(threadID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read turn state: %w", err)
	}
	var state TurnState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal turn state: %w", err)
	}
	return &state, nil
}

// DeleteTurnState removes the active turn state file.
func (s *Store) DeleteTurnState(threadID string) error {
	err := os.Remove(s.turnStatePath(threadID))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete turn state: %w", err)
	}
	return nil
}

// --- Step Result Persistence ---

// stepResultPath returns the path to the step result file.
func (s *Store) stepResultPath(threadID, turnID string, step int) string {
	return filepath.Join(s.turnsDir(threadID), turnID, fmt.Sprintf("step-%03d-result.json", step))
}

// SaveStepResult persists the result of a completed step (assistant message + tool call list).
func (s *Store) SaveStepResult(threadID, turnID string, step int, result StepResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal step result: %w", err)
	}
	return os.WriteFile(s.stepResultPath(threadID, turnID, step), data, 0o644)
}

// LoadStepResult loads a step result from disk. Returns nil if not found.
func (s *Store) LoadStepResult(threadID, turnID string, step int) (*StepResult, error) {
	data, err := os.ReadFile(s.stepResultPath(threadID, turnID, step))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read step result: %w", err)
	}
	var result StepResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal step result: %w", err)
	}
	return &result, nil
}

// --- Tool Results Persistence ---

// toolResultsPath returns the path to the tool results file for a step.
func (s *Store) toolResultsPath(threadID, turnID string, step int) string {
	return filepath.Join(s.turnsDir(threadID), turnID, fmt.Sprintf("step-%03d-tools.json", step))
}

// SaveToolResults persists tool results for a step.
func (s *Store) SaveToolResults(threadID, turnID string, step int, results StepToolResults) error {
	data, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("marshal tool results: %w", err)
	}
	return os.WriteFile(s.toolResultsPath(threadID, turnID, step), data, 0o644)
}

// LoadToolResults loads tool results for a step. Returns empty if not found.
func (s *Store) LoadToolResults(threadID, turnID string, step int) (StepToolResults, error) {
	data, err := os.ReadFile(s.toolResultsPath(threadID, turnID, step))
	if err != nil {
		if os.IsNotExist(err) {
			return StepToolResults{}, nil
		}
		return StepToolResults{}, fmt.Errorf("read tool results: %w", err)
	}
	var results StepToolResults
	if err := json.Unmarshal(data, &results); err != nil {
		return StepToolResults{}, fmt.Errorf("unmarshal tool results: %w", err)
	}
	return results, nil
}

// --- Async Task Persistence ---

// asyncTasksPath returns the path to the async tasks file for a step.
func (s *Store) asyncTasksPath(threadID, turnID string, step int) string {
	return filepath.Join(s.turnsDir(threadID), turnID, fmt.Sprintf("step-%03d-async.json", step))
}

// SaveAsyncTasks persists async task metadata for a step.
func (s *Store) SaveAsyncTasks(threadID, turnID string, step int, tasks StepAsyncTasks) error {
	dir := filepath.Join(s.turnsDir(threadID), turnID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create turn dir: %w", err)
	}
	data, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("marshal async tasks: %w", err)
	}
	return os.WriteFile(s.asyncTasksPath(threadID, turnID, step), data, 0o644)
}

// LoadAsyncTasks loads async task metadata for a step. Returns empty if not found.
func (s *Store) LoadAsyncTasks(threadID, turnID string, step int) (StepAsyncTasks, error) {
	data, err := os.ReadFile(s.asyncTasksPath(threadID, turnID, step))
	if err != nil {
		if os.IsNotExist(err) {
			return StepAsyncTasks{}, nil
		}
		return StepAsyncTasks{}, fmt.Errorf("read async tasks: %w", err)
	}
	var tasks StepAsyncTasks
	if err := json.Unmarshal(data, &tasks); err != nil {
		return StepAsyncTasks{}, fmt.Errorf("unmarshal async tasks: %w", err)
	}
	return tasks, nil
}

// --- Question/Answer Persistence ---

// questionPath returns the path to the pending question file for a turn.
func (s *Store) questionPath(threadID, turnID string) string {
	return filepath.Join(s.turnsDir(threadID), turnID, "question.json")
}

// answerPath returns the path to the answer file for a turn.
func (s *Store) answerPath(threadID, turnID string) string {
	return filepath.Join(s.turnsDir(threadID), turnID, "answer.json")
}

// SaveQuestion persists a pending question to disk.
func (s *Store) SaveQuestion(threadID, turnID string, q PendingQuestionState) error {
	dir := filepath.Join(s.turnsDir(threadID), turnID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create turn dir: %w", err)
	}
	data, err := json.Marshal(q)
	if err != nil {
		return fmt.Errorf("marshal question: %w", err)
	}
	return os.WriteFile(s.questionPath(threadID, turnID), data, 0o644)
}

// LoadQuestion loads a pending question from disk. Returns nil if not found.
func (s *Store) LoadQuestion(threadID, turnID string) (*PendingQuestionState, error) {
	data, err := os.ReadFile(s.questionPath(threadID, turnID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read question: %w", err)
	}
	var q PendingQuestionState
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, fmt.Errorf("unmarshal question: %w", err)
	}
	return &q, nil
}

// SaveAnswer persists the user's answer to disk.
func (s *Store) SaveAnswer(threadID, turnID string, a QuestionAnswer) error {
	dir := filepath.Join(s.turnsDir(threadID), turnID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create turn dir: %w", err)
	}
	data, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshal answer: %w", err)
	}
	return os.WriteFile(s.answerPath(threadID, turnID), data, 0o644)
}

// LoadAnswer loads the user's answer from disk. Returns nil if not found.
func (s *Store) LoadAnswer(threadID, turnID string) (*QuestionAnswer, error) {
	data, err := os.ReadFile(s.answerPath(threadID, turnID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read answer: %w", err)
	}
	var a QuestionAnswer
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("unmarshal answer: %w", err)
	}
	return &a, nil
}

// DeleteQuestionAnswer removes both question and answer files for a turn.
func (s *Store) DeleteQuestionAnswer(threadID, turnID string) {
	os.Remove(s.questionPath(threadID, turnID))
	os.Remove(s.answerPath(threadID, turnID))
}

// --- Compaction Persistence ---

// compactionPath returns the path to the compaction record for a thread.
func (s *Store) compactionPath(threadID string) string {
	return filepath.Join(s.baseDir, threadID, "compaction.json")
}

// SaveCompaction persists a compaction record to disk.
func (s *Store) SaveCompaction(threadID string, record CompactionRecord) error {
	dir := filepath.Join(s.baseDir, threadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create thread dir: %w", err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal compaction: %w", err)
	}
	return os.WriteFile(s.compactionPath(threadID), data, 0o644)
}

// LoadCompaction loads a compaction record from disk. Returns nil if not found.
func (s *Store) LoadCompaction(threadID string) (*CompactionRecord, error) {
	data, err := os.ReadFile(s.compactionPath(threadID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read compaction: %w", err)
	}
	var record CompactionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		// Corrupted file — delete and return nil.
		os.Remove(s.compactionPath(threadID))
		return nil, nil
	}
	return &record, nil
}

// FindLeaf returns the leaf message ID for a thread — the message that is not
// a parent of any other message. Returns "" if the thread has no messages.
func (s *Store) FindLeaf(threadID string) (string, error) {
	dir := s.messagesDir(threadID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read messages dir: %w", err)
	}

	parentIDs := make(map[string]bool)
	var allIDs []string
	for _, e := range entries {
		name := e.Name()
		if len(name) < 6 || name[len(name)-5:] != ".json" {
			continue
		}
		id := name[:len(name)-5]
		allIDs = append(allIDs, id)
		msg, err := s.LoadMessage(threadID, id)
		if err != nil {
			continue
		}
		if msg.ParentID != "" {
			parentIDs[msg.ParentID] = true
		}
	}

	// Find the message that is NOT a parent of any other message.
	for i := len(allIDs) - 1; i >= 0; i-- {
		if !parentIDs[allIDs[i]] {
			return allIDs[i], nil
		}
	}
	return "", nil
}
