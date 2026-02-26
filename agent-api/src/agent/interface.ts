import type { UIMessage, UIMessageChunk } from "ai";
import type { ModelInfo } from "../api/types.js";
import type { CredentialEnvVar } from "../credentials/credentials.js";

/**
 * Agent interface - abstracts the underlying agent implementation.
 *
 * This interface uses AI SDK types (UIMessage, UIMessageChunk) to remain
 * implementation-agnostic. Different implementations handle their own
 * protocol translation, message storage, and lifecycle management internally.
 *
 * The sessionId passed to every method is the Go server's session ID.
 * The agent implementation is responsible for mapping this to its own
 * internal native session ID (e.g., the Claude CLI session ID).
 */
export interface Agent {
	/**
	 * Send a prompt to the agent and stream UIMessageChunk events.
	 * Auto-connects and ensures session exists before prompting.
	 * @param message - The user message to send
	 * @param sessionId - Session ID from the Go server (used as the mapping key)
	 * @param model - Optional model to use for this request. If not provided, uses agent's default.
	 * @param reasoning - Extended thinking: "enabled", "disabled", or undefined for default
	 * @param mode - Permission mode: "plan" for planning mode, or undefined for default (build mode)
	 */
	prompt(
		message: UIMessage,
		sessionId: string,
		model?: string,
		reasoning?: "enabled" | "disabled" | "",
		mode?: "plan" | "",
	): AsyncGenerator<UIMessageChunk, void, unknown>;

	/**
	 * Cancel the current operation.
	 * @param sessionId - Session ID to cancel.
	 */
	cancel(sessionId: string): Promise<void>;

	/**
	 * Update environment variables and credentials.
	 * @param sessionId - Session ID this update is associated with.
	 * @param update - Environment variable key-value pairs
	 * @param credentials - Optional raw credentials with provider info (used by OpenCode's auth.set API)
	 */
	updateEnvironment(
		sessionId: string,
		update: Record<string, string>,
		credentials?: CredentialEnvVar[],
	): Promise<void>;

	/**
	 * List available models.
	 * @param sessionId - Session ID this request is associated with.
	 */
	listModels(sessionId: string): Promise<ModelInfo[]>;

	/**
	 * Get all messages for a session.
	 * Auto-connects and ensures the session exists before loading messages.
	 * @param sessionId - Session ID to fetch messages for.
	 */
	getMessages(sessionId: string): Promise<UIMessage[]>;
}
