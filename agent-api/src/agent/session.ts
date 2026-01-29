import type { UIMessage } from "ai";

/**
 * Session interface - represents an individual chat session with its own message history.
 *
 * Each session is independent and maintains its own state. The agent manages multiple
 * sessions and can switch between them.
 */
export interface Session {
	/**
	 * Unique session identifier.
	 */
	readonly id: string;

	/**
	 * Get all messages in this session.
	 */
	getMessages(): UIMessage[];

	/**
	 * Add a message to this session.
	 */
	addMessage(message: UIMessage): void;

	/**
	 * Update an existing message by ID.
	 */
	updateMessage(id: string, updates: Partial<UIMessage>): void;

	/**
	 * Get the last assistant message (for updating during streaming).
	 */
	getLastAssistantMessage(): UIMessage | undefined;

	/**
	 * Clear all messages in this session.
	 */
	clearMessages(): void;
}
