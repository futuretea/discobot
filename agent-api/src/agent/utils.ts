import type { UIMessage } from "ai";

/** Type alias for UIMessage parts (extracted from UIMessage to avoid generic params) */
type MessagePart = UIMessage["parts"][number];

/**
 * Generate a unique message ID
 */
export function generateMessageId(): string {
	return `msg-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

/**
 * Create a new UIMessage with an optional array of parts
 */
export function createUIMessage(
	role: "user" | "assistant",
	parts: MessagePart[] = [],
): UIMessage {
	return {
		id: generateMessageId(),
		role,
		parts,
	};
}
