import type {
	ContentBlock,
	SessionUpdate,
	ToolCall,
	ToolCallUpdate,
} from "@agentclientprotocol/sdk";
import type {
	DynamicToolUIPart,
	ReasoningUIPart,
	TextUIPart,
	UIMessage,
} from "ai";

// Union of the part types we actually produce
type ProducedUIPart = TextUIPart | ReasoningUIPart | DynamicToolUIPart;

/**
 * Convert UIMessage parts to ACP ContentBlock array
 */
export function uiMessageToContentBlocks(message: UIMessage): ContentBlock[] {
	const blocks: ContentBlock[] = [];

	for (const part of message.parts) {
		if (part.type === "text") {
			blocks.push({
				type: "text",
				text: part.text,
			});
		} else if (part.type === "file") {
			blocks.push({
				type: "resource_link",
				uri: part.url,
				name: part.filename || "file",
				mimeType: part.mediaType,
			});
		}
	}

	return blocks;
}

/**
 * Convert ACP SessionUpdate to a UIMessagePart
 */
export function sessionUpdateToUIPart(
	update: SessionUpdate,
): ProducedUIPart | null {
	switch (update.sessionUpdate) {
		case "agent_message_chunk":
			if (update.content.type === "text") {
				return {
					type: "text",
					text: update.content.text,
				} satisfies TextUIPart;
			}
			break;

		case "agent_thought_chunk":
			if (update.content.type === "text") {
				return {
					type: "reasoning",
					text: update.content.text,
				} satisfies ReasoningUIPart;
			}
			break;

		case "tool_call":
			return toolCallToUIPart(update);

		case "tool_call_update":
			return toolCallUpdateToUIPart(update);
	}

	return null;
}

/**
 * Convert ACP ToolCall to DynamicToolUIPart
 */
export function toolCallToUIPart(toolCall: ToolCall): DynamicToolUIPart {
	const state = toolCallStatusToState(toolCall.status);

	if (state === "output-error") {
		return {
			type: "dynamic-tool",
			toolCallId: toolCall.toolCallId,
			toolName: toolCall.title,
			state: "output-error",
			input: toolCall.rawInput || {},
			errorText: String(toolCall.rawOutput || "Tool call failed"),
		};
	}

	if (state === "output-available") {
		return {
			type: "dynamic-tool",
			toolCallId: toolCall.toolCallId,
			toolName: toolCall.title,
			state: "output-available",
			input: toolCall.rawInput || {},
			output: toolCall.rawOutput,
		};
	}

	if (state === "input-available") {
		return {
			type: "dynamic-tool",
			toolCallId: toolCall.toolCallId,
			toolName: toolCall.title,
			state: "input-available",
			input: toolCall.rawInput || {},
		};
	}

	// input-streaming
	return {
		type: "dynamic-tool",
		toolCallId: toolCall.toolCallId,
		toolName: toolCall.title,
		state: "input-streaming",
		input: toolCall.rawInput,
	};
}

/**
 * Convert ACP ToolCallUpdate to DynamicToolUIPart
 */
export function toolCallUpdateToUIPart(
	update: ToolCallUpdate,
): DynamicToolUIPart {
	const state = toolCallStatusToState(update.status);

	if (state === "output-error") {
		return {
			type: "dynamic-tool",
			toolCallId: update.toolCallId,
			toolName: update.title || "unknown",
			state: "output-error",
			input: update.rawInput || {},
			errorText: String(update.rawOutput || "Tool call failed"),
		};
	}

	if (state === "output-available") {
		return {
			type: "dynamic-tool",
			toolCallId: update.toolCallId,
			toolName: update.title || "unknown",
			state: "output-available",
			input: update.rawInput || {},
			output: update.rawOutput,
		};
	}

	if (state === "input-available") {
		return {
			type: "dynamic-tool",
			toolCallId: update.toolCallId,
			toolName: update.title || "unknown",
			state: "input-available",
			input: update.rawInput || {},
		};
	}

	// input-streaming
	return {
		type: "dynamic-tool",
		toolCallId: update.toolCallId,
		toolName: update.title || "unknown",
		state: "input-streaming",
		input: update.rawInput,
	};
}

function toolCallStatusToState(
	status?: "pending" | "in_progress" | "completed" | "failed" | null,
): "input-streaming" | "input-available" | "output-available" | "output-error" {
	switch (status) {
		case "completed":
			return "output-available";
		case "failed":
			return "output-error";
		case "in_progress":
			return "input-available";
		default:
			return "input-streaming";
	}
}

/**
 * Generate a unique message ID
 */
export function generateMessageId(): string {
	return `msg-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

/**
 * Create a new UIMessage
 */
export function createUIMessage(
	role: "user" | "assistant",
	parts: ProducedUIPart[] = [],
): UIMessage {
	return {
		id: generateMessageId(),
		role,
		parts,
	};
}
