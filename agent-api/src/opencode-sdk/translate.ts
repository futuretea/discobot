/**
 * OpenCode SDK Message Translation
 *
 * Handles translation between:
 * - UIMessage <-> OpenCode Parts
 * - OpenCode Events -> UIMessageChunks (stateful streaming translation)
 */

import type {
	AgentPartInput,
	Event,
	FilePart,
	FilePartInput,
	Message,
	Part,
	ReasoningPart,
	SubtaskPartInput,
	TextPart,
	TextPartInput,
	ToolPart,
} from "@opencode-ai/sdk/v2";
import type { UIMessage, UIMessageChunk } from "ai";

/**
 * Translate UIMessage to OpenCode parts format
 */
export function translateUIMessageToParts(
	message: UIMessage,
): Array<TextPartInput | FilePartInput | AgentPartInput | SubtaskPartInput> {
	const parts: Array<
		TextPartInput | FilePartInput | AgentPartInput | SubtaskPartInput
	> = [];

	for (const part of message.parts) {
		if (part.type === "text") {
			parts.push({
				type: "text",
				text: part.text,
			});
		} else if (part.type === "file" && part.url) {
			// Handle file parts (images, etc.)
			const filename = part.url.split("/").pop() || "file";
			parts.push({
				type: "file",
				mime: "application/octet-stream", // Could be improved with actual MIME detection
				filename,
				url: part.url,
			});
		}
		// Tool results are handled separately by OpenCode
	}

	return parts;
}

/**
 * Translate an OpenCode message (info + parts) to a UIMessage.
 * Used by OpenCodeSession.load() to populate the message cache from the server.
 */
export function translateOpenCodeMessageToUIMessage(
	info: Message,
	parts: Part[],
): UIMessage {
	const uiParts: UIMessage["parts"] = [];

	for (const part of parts) {
		switch (part.type) {
			case "text": {
				const textPart = part as TextPart;
				if (textPart.text) {
					uiParts.push({ type: "text", text: textPart.text });
				}
				break;
			}
			case "reasoning": {
				const reasoningPart = part as ReasoningPart;
				if (reasoningPart.text) {
					uiParts.push({
						type: "reasoning",
						text: reasoningPart.text,
					});
				}
				break;
			}
			case "tool": {
				const toolPart = part as ToolPart;
				const state = toolPart.state;

				if (state.status === "completed") {
					uiParts.push({
						type: "dynamic-tool",
						toolCallId: toolPart.callID,
						toolName: toolPart.tool,
						state: "output-available",
						input: state.input ?? {},
						output: state.output,
					} as UIMessage["parts"][number]);
				} else if (state.status === "error") {
					uiParts.push({
						type: "dynamic-tool",
						toolCallId: toolPart.callID,
						toolName: toolPart.tool,
						state: "output-error",
						input: state.input ?? {},
						errorText: state.error,
					} as UIMessage["parts"][number]);
				} else {
					uiParts.push({
						type: "dynamic-tool",
						toolCallId: toolPart.callID,
						toolName: toolPart.tool,
						state: "input-available",
						input: state.input ?? {},
					} as UIMessage["parts"][number]);
				}
				break;
			}
			case "file": {
				const filePart = part as FilePart;
				if (filePart.url) {
					uiParts.push({
						type: "file",
						url: filePart.url,
						mediaType: filePart.mime || "application/octet-stream",
					} as UIMessage["parts"][number]);
				}
				break;
			}
			// step-start and step-finish are streaming lifecycle markers.
			// For replay they add no value (and cause empty messages for
			// aborted turns that only have step-starts with no content).
		}
	}

	return {
		id: info.id,
		role: info.role as "user" | "assistant",
		parts: uiParts,
	};
}

/**
 * State tracked across events during a single prompt() streaming session.
 * Required because OpenCode sends events out-of-order relative to what
 * the UIMessageChunk protocol expects (e.g., deltas arrive before the
 * part-updated that would signal text-start).
 */
export interface StreamTranslationState {
	/** The assistant message ID we're currently streaming. */
	assistantMessageId: string | null;
	/** Part IDs for which we've already emitted a text-start or reasoning-start. */
	startedParts: Set<string>;
	/** Part IDs for which we've already emitted tool-input-start. */
	startedToolParts: Set<string>;
	/** Part IDs known to be reasoning parts (so deltas route to reasoning-delta, not text-delta). */
	reasoningPartIds: Set<string>;
}

export function createStreamTranslationState(): StreamTranslationState {
	return {
		assistantMessageId: null,
		startedParts: new Set(),
		startedToolParts: new Set(),
		reasoningPartIds: new Set(),
	};
}

/**
 * Translate an OpenCode SSE event into UIMessageChunks.
 *
 * This is STATEFUL — it mutates `state` to track the assistant message ID
 * and which parts have been started. This is necessary because:
 * - Deltas can arrive before the part-updated that creates the part
 * - User message parts/deltas must be filtered out
 * - Step events come as real parts from OpenCode, not synthesized
 */
export function translateEventsToChunks(
	event: Event,
	opencodeSessionId: string,
	state: StreamTranslationState,
): UIMessageChunk[] {
	const chunks: UIMessageChunk[] = [];

	switch (event.type) {
		case "message.updated": {
			const msg = event.properties.info;

			// Only process messages for our session
			if (msg.sessionID !== opencodeSessionId) {
				break;
			}

			// Only process assistant messages
			if (msg.role !== "assistant") {
				break;
			}

			// Track the assistant message ID so we can filter parts/deltas
			if (!state.assistantMessageId) {
				state.assistantMessageId = msg.id;
			}

			// Emit start when we first see the assistant message (no completed)
			if (!msg.time.completed) {
				chunks.push({
					type: "start",
					messageId: msg.id,
				});
			}

			// Emit finish when the message is completed
			if (msg.time.completed) {
				chunks.push({
					type: "finish",
					finishReason:
						(msg.finish as "stop" | "length" | "error" | undefined) || "stop",
				});
			}

			break;
		}

		case "message.part.updated": {
			const part = event.properties.part;

			// Only process parts for our session
			if (part.sessionID !== opencodeSessionId) {
				break;
			}

			// Only process parts belonging to the current assistant message.
			// This filters out user message parts (e.g., the user's text part).
			if (part.messageID !== state.assistantMessageId) {
				break;
			}

			switch (part.type) {
				case "step-start": {
					chunks.push({ type: "start-step" });
					break;
				}
				case "step-finish": {
					chunks.push({ type: "finish-step" });
					break;
				}
				case "text": {
					const textPart = part as TextPart;
					if (!textPart.time?.end) {
						// Part just created — streaming will happen via deltas.
						// Emit text-start if we haven't already.
						if (!state.startedParts.has(part.id)) {
							state.startedParts.add(part.id);
							chunks.push({ type: "text-start", id: part.id });
						}
					} else {
						// Part completed (has time.end).
						// If we never saw a start for this part (no deltas came),
						// emit the full content as a single text block.
						if (!state.startedParts.has(part.id)) {
							state.startedParts.add(part.id);
							chunks.push({ type: "text-start", id: part.id });
							if (textPart.text) {
								chunks.push({
									type: "text-delta",
									id: part.id,
									delta: textPart.text,
								});
							}
						}
						chunks.push({ type: "text-end", id: part.id });
					}
					break;
				}
				case "reasoning": {
					const reasoningPart = part as ReasoningPart;
					// Track this as a reasoning part so deltas route correctly
					// (OpenCode sends field:"text" for reasoning deltas too)
					state.reasoningPartIds.add(part.id);
					if (!reasoningPart.time?.end) {
						if (!state.startedParts.has(part.id)) {
							state.startedParts.add(part.id);
							chunks.push({ type: "reasoning-start", id: part.id });
						}
					} else {
						if (!state.startedParts.has(part.id)) {
							state.startedParts.add(part.id);
							chunks.push({ type: "reasoning-start", id: part.id });
							if (reasoningPart.text) {
								chunks.push({
									type: "reasoning-delta",
									id: part.id,
									delta: reasoningPart.text,
								});
							}
						}
						chunks.push({ type: "reasoning-end", id: part.id });
					}
					break;
				}
				case "tool": {
					const toolPart = part as ToolPart;
					const toolState = toolPart.state;

					// Emit tool-input-start only once per tool part
					if (!state.startedToolParts.has(part.id)) {
						state.startedToolParts.add(part.id);
						chunks.push({
							type: "tool-input-start",
							toolCallId: toolPart.callID,
							toolName: toolPart.tool,
							dynamic: true,
						});
					}

					if (toolState.input) {
						chunks.push({
							type: "tool-input-available",
							toolCallId: toolPart.callID,
							toolName: toolPart.tool,
							input: toolState.input,
							dynamic: true,
						});
					}

					if (toolState.status === "completed") {
						chunks.push({
							type: "tool-output-available",
							toolCallId: toolPart.callID,
							output: toolState.output,
							dynamic: true,
						});
					} else if (toolState.status === "error") {
						chunks.push({
							type: "tool-output-error",
							toolCallId: toolPart.callID,
							errorText: toolState.error,
							dynamic: true,
						});
					}
					break;
				}
				case "file": {
					const filePart = part as FilePart;
					chunks.push({ type: "text-start", id: part.id });
					chunks.push({
						type: "text-delta",
						id: part.id,
						delta: `[File: ${filePart.filename || filePart.url}]`,
					});
					chunks.push({ type: "text-end", id: part.id });
					break;
				}
				// snapshot, patch, agent, retry, compaction, subtask — ignore
			}

			break;
		}

		case "message.part.delta": {
			const { sessionID, messageID, partID, delta, field } = event.properties;

			// Only process deltas for our session
			if (sessionID !== opencodeSessionId) {
				break;
			}

			// Only process deltas for the current assistant message.
			// OpenCode sends deltas for user messages too — ignore those.
			if (messageID !== state.assistantMessageId) {
				break;
			}

			if (!delta) {
				break;
			}

			// OpenCode sends field:"text" for both text and reasoning parts
			// (because ReasoningPart stores content in its `text` property).
			// Use reasoningPartIds to distinguish.
			const isReasoning = state.reasoningPartIds.has(partID);

			if (isReasoning) {
				if (!state.startedParts.has(partID)) {
					state.startedParts.add(partID);
					chunks.push({ type: "reasoning-start", id: partID });
				}
				chunks.push({ type: "reasoning-delta", id: partID, delta });
			} else if (field === "text") {
				// Emit text-start if this is the first delta for this part
				// (the part-updated with empty text may not have arrived yet)
				if (!state.startedParts.has(partID)) {
					state.startedParts.add(partID);
					chunks.push({ type: "text-start", id: partID });
				}
				chunks.push({ type: "text-delta", id: partID, delta });
			}
			break;
		}

		// Other event types we don't need to handle
		default:
			break;
	}

	return chunks;
}
