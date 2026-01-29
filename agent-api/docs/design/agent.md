# Agent Interface

The Agent interface provides an abstraction layer over different agent protocol implementations, allowing the agent-api to work with ACP-based agents as well as other types of agents (HTTP-based, custom protocols, etc.).

## Overview

Previously, the agent-api was tightly coupled to the ACP (Agent Client Protocol) implementation. The new `Agent` interface abstracts away the protocol details, making it possible to:

1. Support non-ACP agent implementations
2. Swap out agent implementations without changing the rest of the codebase
3. Control message persistence on a per-implementation basis

## Interface Definition

The Agent interface uses AI SDK types (UIMessage, UIMessageChunk) to remain protocol-agnostic:

```typescript
export interface Agent {
  // Connection management
  connect(): Promise<void>;
  ensureSession(): Promise<string>;
  disconnect(): Promise<void>;
  get isConnected(): boolean;

  // Messaging
  setUpdateCallback(callback: AgentUpdateCallback | null): void;
  prompt(message: UIMessage): Promise<void>;
  cancel(): Promise<void>;

  // Environment
  updateEnvironment(update: EnvironmentUpdate): Promise<void>;
  getEnvironment(): Record<string, string>;

  // Message storage - agent owns the message history
  getMessages(): UIMessage[];
  addMessage(message: UIMessage): void;
  updateMessage(id: string, updates: Partial<UIMessage>): void;
  getLastAssistantMessage(): UIMessage | undefined;
  clearMessages(): void;
  clearSession(): Promise<void>;
}

export type AgentUpdateCallback = (chunk: UIMessageChunk) => void;
```

### Methods

**Connection Management:**
- **`connect()`**: Establish a connection to the agent. For ACP, this spawns the child process and initializes the ACP connection.
- **`ensureSession()`**: Ensure a session exists, creating a new one or resuming an existing session. Returns the session ID.
- **`disconnect()`**: Clean up resources and disconnect from the agent.
- **`isConnected`**: Check if the agent is currently connected.

**Messaging:**
- **`setUpdateCallback(callback)`**: Register a callback to receive UIMessageChunk events for SSE streaming.
- **`prompt(message)`**: Send a UIMessage to the agent. The agent translates to its protocol format internally.
- **`cancel()`**: Cancel the current operation.

**Environment:**
- **`updateEnvironment(update)`**: Update environment variables and restart the agent if connected.
- **`getEnvironment()`**: Get current environment variables.

**Message Storage:**
- **`getMessages()`**: Get all messages in the current session.
- **`addMessage(message)`**: Add a message to the session.
- **`updateMessage(id, updates)`**: Update an existing message by ID.
- **`getLastAssistantMessage()`**: Get the last assistant message (for updating during streaming).
- **`clearMessages()`**: Clear all messages in the session.
- **`clearSession()`**: Clear the session completely (messages and session state).

## Key Design Principles

### 1. Protocol Agnostic

The interface uses **AI SDK types** (UIMessage, UIMessageChunk) rather than protocol-specific types (ACP's ContentBlock, SessionNotification). This means:

- **Implementations translate internally**: The ACPClient translates UIMessage → ContentBlock when sending, and SessionUpdate → UIMessageChunk when receiving.
- **Easy to add new protocols**: An HTTP-based agent would translate UIMessage → HTTP JSON payload, and HTTP SSE events → UIMessageChunk.
- **Clean boundaries**: The rest of the codebase only sees AI SDK types.

### 2. Agent Owns Message Storage

The Agent interface includes message storage methods. This means:

- **Each implementation controls persistence**: ACP agent can save to disk, HTTP agent can sync with server, etc.
- **No shared store pollution**: Different agent instances have isolated message stores.
- **Simpler completion logic**: The completion handler just forwards chunks to SSE; the agent handles accumulating messages internally.

### 3. Streaming via Chunks

The `setUpdateCallback` receives **UIMessageChunk** events, not protocol-specific updates:

- **Unified streaming protocol**: All agents produce the same chunk format for SSE streaming.
- **Agent handles translation**: ACPClient uses `sessionUpdateToChunks()` to translate ACP updates.
- **Simple SSE handler**: The completion just forwards chunks to `addCompletionEvent()`.

## ACP Implementation

The `ACPClient` class implements the `Agent` interface and handles:

- **Protocol translation**: UIMessage ↔ ACP ContentBlock, SessionUpdate → UIMessageChunk
- **Process management**: Spawns the ACP agent as a child process
- **Stdio streams**: Manages JSON-RPC communication over stdin/stdout
- **Session lifecycle**: New, resume, and load session strategies
- **Message accumulation**: Updates in-memory message store from ACP SessionUpdates
- **Message replay**: Reconstructs messages during session recovery
- **Auto-approval**: Automatically approves permission requests

### Message Persistence

The ACP implementation includes a `persistMessages` flag to control whether messages are saved to disk:

```typescript
export interface ACPClientOptions {
  command: string;
  args?: string[];
  cwd: string;
  env?: Record<string, string>;
  persistMessages?: boolean; // Default: false
}
```

**Why is this needed?**

Different ACP implementations handle session recovery differently:

1. **Claude Code ACP**: Uses `unstable_resumeSession()` which reconnects to an existing session WITHOUT replaying messages. This requires persisting messages to disk so they can be loaded on resume.

2. **Standard ACP**: Uses `loadSession()` which replays all messages from the agent's storage. No need for disk persistence.

The `persistMessages` flag allows the ACP client to adapt to both scenarios.

### Session Recovery Strategy

The ACP client tries three strategies in order:

1. **`unstable_resumeSession()`** (experimental)
   - Reconnects to existing session without message replay
   - Used by Claude Code ACP
   - Requires `persistMessages: true`

2. **`loadSession()`** (standard)
   - Creates new ACP session and replays messages
   - Captures replayed messages and stores them in memory
   - Works with any ACP implementation that supports `loadSession` capability

3. **`newSession()`** (fallback)
   - Creates a fresh session
   - Clears all messages

## Configuration

The message persistence flag can be controlled via environment variable:

```bash
# Disable message persistence (for agents that replay messages)
PERSIST_MESSAGES=false

# Enable message persistence (default, for Claude Code ACP)
PERSIST_MESSAGES=true
```

In the application code:

```typescript
const agent: Agent = new ACPClient({
  command: "claude-code-acp",
  args: [],
  cwd: "/workspace",
  persistMessages: true, // Enable for Claude Code ACP
});
```

## Usage in App

The `createApp()` function accepts an `Agent` implementation:

```typescript
export function createApp(options: AppOptions) {
  const agent: Agent = new ACPClient({
    command: options.agentCommand,
    args: options.agentArgs,
    cwd: options.agentCwd,
    persistMessages: options.persistMessages ?? true,
  });

  // ... use agent throughout the app
}
```

All other modules (`completion.ts`, `app.ts`) now depend only on the `Agent` interface, not the concrete `ACPClient` implementation.

## Adding New Agent Implementations

To add a new agent implementation:

1. Create a new class that implements the `Agent` interface
2. Implement all required methods
3. Handle session management appropriate to your protocol
4. Instantiate your implementation in `createApp()` instead of `ACPClient`

Example:

```typescript
class HttpAgent implements Agent {
  constructor(private baseUrl: string) {}

  async connect(): Promise<void> {
    // Establish HTTP connection
  }

  async ensureSession(): Promise<string> {
    // Create or resume session via HTTP API
  }

  // ... implement other methods
}

// In createApp:
const agent: Agent = new HttpAgent("http://localhost:8080");
```

## Design Decisions

### Translation Flow

**Sending a prompt:**
```
UIMessage (AI SDK)
  → Agent.prompt(message)
  → ACPClient: uiMessageToContentBlocks(message)
  → ContentBlock[] (ACP)
  → ACP prompt() call
```

**Receiving updates:**
```
SessionUpdate (ACP)
  → ACPClient sessionUpdate handler
  → updateMessageFromACP(update)  # Updates message store
  → sessionUpdateToChunks(update)  # Generates chunks
  → UIMessageChunk[] (AI SDK)
  → updateCallback(chunk)  # Forwards to SSE
```

**Message Storage:**
```
SessionUpdate (ACP)
  → extract text/reasoning/tool data
  → update last assistant message parts
  → updateMessage(id, { parts })
  → store module persists to disk (if enabled)
```

## Benefits

1. **Decoupling**: The app no longer depends on ACP-specific implementation details
2. **Flexibility**: Easy to add new agent types (HTTP, gRPC, WebSocket, etc.)
3. **Testability**: Easy to create mock agents for testing
4. **Configuration**: Per-implementation control over features like message persistence

## Related Modules

- [ACP Module](./acp.md) - ACP implementation of the Agent interface
- [Server Module](./server.md) - Uses the Agent interface for completion handling
- [Store Module](./store.md) - Manages session and message storage
