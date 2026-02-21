import assert from "node:assert";
import { beforeEach, describe, it } from "node:test";
import type { Event, Message, Part } from "@opencode-ai/sdk/v2";
import type { UIMessage } from "ai";
import {
	createStreamTranslationState,
	type StreamTranslationState,
	translateEventsToChunks,
	translateOpenCodeMessageToUIMessage,
	translateUIMessageToParts,
} from "./translate.js";

// Helpers to cast test fixtures (bypasses strict type checks for test data)
function asEvent(event: unknown): Event {
	return event as Event;
}

function asMessage(msg: unknown): Message {
	return msg as Message;
}

function asParts(parts: unknown[]): Part[] {
	return parts as Part[];
}

// Common test IDs
const SESSION_ID = "session-1";
const MSG_ID = "msg-assistant-1";
const USER_MSG_ID = "msg-user-1";

describe("opencode translate", () => {
	// ─── translateUIMessageToParts ───────────────────────────────────

	describe("translateUIMessageToParts", () => {
		it("translates a text part", () => {
			const message: UIMessage = {
				id: "ui-1",
				role: "user",
				parts: [{ type: "text", text: "Hello world" }],
			};

			const parts = translateUIMessageToParts(message);
			assert.strictEqual(parts.length, 1);
			assert.deepStrictEqual(parts[0], {
				type: "text",
				text: "Hello world",
			});
		});

		it("translates a file part with URL", () => {
			const message: UIMessage = {
				id: "ui-1",
				role: "user",
				parts: [
					{
						type: "file",
						url: "https://example.com/images/photo.png",
						mediaType: "image/png",
					},
				],
			};

			const parts = translateUIMessageToParts(message);
			assert.strictEqual(parts.length, 1);
			assert.deepStrictEqual(parts[0], {
				type: "file",
				mime: "application/octet-stream",
				filename: "photo.png",
				url: "https://example.com/images/photo.png",
			});
		});

		it("uses 'file' as fallback filename when URL has no path", () => {
			const message: UIMessage = {
				id: "ui-1",
				role: "user",
				parts: [
					{
						type: "file",
						url: "",
						mediaType: "image/png",
					},
				],
			};

			const parts = translateUIMessageToParts(message);
			// Empty URL means no url property → part.url is falsy → skipped
			assert.strictEqual(parts.length, 0);
		});

		it("translates multiple parts", () => {
			const message: UIMessage = {
				id: "ui-1",
				role: "user",
				parts: [
					{ type: "text", text: "Check this image:" },
					{
						type: "file",
						url: "https://example.com/img.jpg",
						mediaType: "image/jpeg",
					},
				],
			};

			const parts = translateUIMessageToParts(message);
			assert.strictEqual(parts.length, 2);
			assert.strictEqual(parts[0].type, "text");
			assert.strictEqual(parts[1].type, "file");
		});

		it("skips dynamic-tool parts", () => {
			const message: UIMessage = {
				id: "ui-1",
				role: "assistant",
				parts: [
					{ type: "text", text: "Let me read that file" },
					{
						type: "dynamic-tool",
						toolCallId: "tool-1",
						toolName: "Read",
						state: "output-available",
						input: { file_path: "/test.txt" },
						output: "file contents",
					},
				],
			};

			const parts = translateUIMessageToParts(message);
			assert.strictEqual(parts.length, 1);
			assert.strictEqual(parts[0].type, "text");
		});

		it("returns empty array for message with no translatable parts", () => {
			const message: UIMessage = {
				id: "ui-1",
				role: "assistant",
				parts: [],
			};

			const parts = translateUIMessageToParts(message);
			assert.strictEqual(parts.length, 0);
		});
	});

	// ─── translateOpenCodeMessageToUIMessage ─────────────────────────

	describe("translateOpenCodeMessageToUIMessage", () => {
		it("translates a text part", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000, completed: 2000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "text",
					text: "Hello!",
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);

			assert.strictEqual(uiMessage.id, MSG_ID);
			assert.strictEqual(uiMessage.role, "assistant");
			assert.strictEqual(uiMessage.parts.length, 1);
			assert.deepStrictEqual(uiMessage.parts[0], {
				type: "text",
				text: "Hello!",
			});
		});

		it("skips text parts with empty text", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "text",
					text: "",
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 0);
		});

		it("translates a reasoning part", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "reasoning",
					text: "Let me think about this...",
					time: { start: 1000, end: 2000 },
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 1);
			assert.deepStrictEqual(uiMessage.parts[0], {
				type: "reasoning",
				text: "Let me think about this...",
			});
		});

		it("skips reasoning parts with empty text", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "reasoning",
					text: "",
					time: { start: 1000, end: 2000 },
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 0);
		});

		it("translates a completed tool part", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "tool",
					callID: "call-1",
					tool: "Read",
					state: {
						status: "completed",
						input: { file_path: "/test.txt" },
						output: "file contents",
						title: "Read /test.txt",
						metadata: {},
						time: { start: 1000, end: 2000 },
					},
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 1);
			const part = uiMessage.parts[0] as {
				type: string;
				toolCallId: string;
				toolName: string;
				state: string;
				input: unknown;
				output: string;
			};
			assert.strictEqual(part.type, "dynamic-tool");
			assert.strictEqual(part.toolCallId, "call-1");
			assert.strictEqual(part.toolName, "Read");
			assert.strictEqual(part.state, "output-available");
			assert.strictEqual(part.output, "file contents");
		});

		it("translates an error tool part", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "tool",
					callID: "call-1",
					tool: "Bash",
					state: {
						status: "error",
						input: { command: "rm -rf /" },
						error: "Permission denied",
						time: { start: 1000, end: 2000 },
					},
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 1);
			const part = uiMessage.parts[0] as {
				type: string;
				state: string;
				errorText: string;
			};
			assert.strictEqual(part.type, "dynamic-tool");
			assert.strictEqual(part.state, "output-error");
			assert.strictEqual(part.errorText, "Permission denied");
		});

		it("translates a running tool part as input-available", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "tool",
					callID: "call-1",
					tool: "Bash",
					state: {
						status: "running",
						input: { command: "ls" },
						time: { start: 1000 },
					},
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 1);
			const part = uiMessage.parts[0] as {
				type: string;
				state: string;
				input: unknown;
			};
			assert.strictEqual(part.type, "dynamic-tool");
			assert.strictEqual(part.state, "input-available");
		});

		it("translates a file part", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "file",
					mime: "image/png",
					url: "https://example.com/image.png",
					filename: "image.png",
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 1);
			const part = uiMessage.parts[0] as {
				type: string;
				url: string;
				mediaType: string;
			};
			assert.strictEqual(part.type, "file");
			assert.strictEqual(part.url, "https://example.com/image.png");
			assert.strictEqual(part.mediaType, "image/png");
		});

		it("skips file parts with no URL", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "file",
					mime: "image/png",
					url: "",
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 0);
		});

		it("skips step-start and step-finish parts (streaming lifecycle only)", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "step-start",
				},
				{
					id: "part-2",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "text",
					text: "Hello",
				},
				{
					id: "part-3",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "step-finish",
					reason: "done",
					cost: 0.01,
					tokens: {
						input: 100,
						output: 50,
						reasoning: 0,
						cache: { read: 0, write: 0 },
					},
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			// Only the text part should survive — step-start/step-finish are skipped
			assert.strictEqual(uiMessage.parts.length, 1);
			assert.strictEqual(uiMessage.parts[0].type, "text");
		});

		it("handles multiple parts of different types", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "reasoning",
					text: "thinking...",
					time: { start: 1000, end: 1500 },
				},
				{
					id: "part-2",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "text",
					text: "Response text",
				},
				{
					id: "part-3",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "tool",
					callID: "call-1",
					tool: "Read",
					state: {
						status: "completed",
						input: { file_path: "/x" },
						output: "content",
						title: "Read /x",
						metadata: {},
						time: { start: 1000, end: 2000 },
					},
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 3);
			assert.strictEqual(uiMessage.parts[0].type, "reasoning");
			assert.strictEqual(uiMessage.parts[1].type, "text");
			assert.strictEqual(uiMessage.parts[2].type, "dynamic-tool");
		});

		it("returns empty parts for unknown part types", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "snapshot",
					snapshot: "abc123",
				},
				{
					id: "part-2",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "patch",
					hash: "def456",
					files: [],
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 0);
		});

		it("preserves user role from message info", () => {
			const info = asMessage({
				id: USER_MSG_ID,
				sessionID: SESSION_ID,
				role: "user",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: USER_MSG_ID,
					type: "text",
					text: "User question",
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.role, "user");
		});

		it("uses default mime type when file part has no mime", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "file",
					mime: "",
					url: "https://example.com/file.bin",
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			assert.strictEqual(uiMessage.parts.length, 1);
			const part = uiMessage.parts[0] as { mediaType: string };
			assert.strictEqual(part.mediaType, "application/octet-stream");
		});

		it("handles tool part with null input as empty object", () => {
			const info = asMessage({
				id: MSG_ID,
				sessionID: SESSION_ID,
				role: "assistant",
				time: { created: 1000 },
			});
			const parts = asParts([
				{
					id: "part-1",
					sessionID: SESSION_ID,
					messageID: MSG_ID,
					type: "tool",
					callID: "call-1",
					tool: "Bash",
					state: {
						status: "completed",
						input: null,
						output: "ok",
						title: "Bash",
						metadata: {},
						time: { start: 1000, end: 2000 },
					},
				},
			]);

			const uiMessage = translateOpenCodeMessageToUIMessage(info, parts);
			const part = uiMessage.parts[0] as { input: unknown };
			assert.deepStrictEqual(part.input, {});
		});
	});

	// ─── translateEventsToChunks (stateful streaming) ────────────────

	describe("translateEventsToChunks", () => {
		let state: StreamTranslationState;

		beforeEach(() => {
			state = createStreamTranslationState();
		});

		// ── message.updated ─────────────────────────────────────────

		describe("message.updated", () => {
			it("emits start for new assistant message (no completed time)", () => {
				const event = asEvent({
					type: "message.updated",
					properties: {
						info: {
							id: MSG_ID,
							sessionID: SESSION_ID,
							role: "assistant",
							time: { created: 1000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);

				assert.strictEqual(chunks.length, 1);
				assert.strictEqual(chunks[0].type, "start");
				assert.strictEqual(
					(chunks[0] as { messageId: string }).messageId,
					MSG_ID,
				);
			});

			it("tracks assistant message ID in state", () => {
				const event = asEvent({
					type: "message.updated",
					properties: {
						info: {
							id: MSG_ID,
							sessionID: SESSION_ID,
							role: "assistant",
							time: { created: 1000 },
						},
					},
				});

				translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(state.assistantMessageId, MSG_ID);
			});

			it("emits finish when message has completed time", () => {
				state.assistantMessageId = MSG_ID;

				const event = asEvent({
					type: "message.updated",
					properties: {
						info: {
							id: MSG_ID,
							sessionID: SESSION_ID,
							role: "assistant",
							time: { created: 1000, completed: 2000 },
							finish: "stop",
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);

				const finish = chunks.find((c) => c.type === "finish");
				assert.ok(finish, "Should emit finish");
				assert.strictEqual(
					(finish as { finishReason: string }).finishReason,
					"stop",
				);
			});

			it("defaults finishReason to 'stop' when not provided", () => {
				state.assistantMessageId = MSG_ID;

				const event = asEvent({
					type: "message.updated",
					properties: {
						info: {
							id: MSG_ID,
							sessionID: SESSION_ID,
							role: "assistant",
							time: { created: 1000, completed: 2000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				const finish = chunks.find((c) => c.type === "finish");
				assert.ok(finish);
				assert.strictEqual(
					(finish as { finishReason: string }).finishReason,
					"stop",
				);
			});

			it("passes through finish reason from message", () => {
				state.assistantMessageId = MSG_ID;

				const event = asEvent({
					type: "message.updated",
					properties: {
						info: {
							id: MSG_ID,
							sessionID: SESSION_ID,
							role: "assistant",
							time: { created: 1000, completed: 2000 },
							finish: "length",
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				const finish = chunks.find((c) => c.type === "finish");
				assert.strictEqual(
					(finish as { finishReason: string }).finishReason,
					"length",
				);
			});

			it("ignores user messages", () => {
				const event = asEvent({
					type: "message.updated",
					properties: {
						info: {
							id: USER_MSG_ID,
							sessionID: SESSION_ID,
							role: "user",
							time: { created: 1000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
				assert.strictEqual(state.assistantMessageId, null);
			});

			it("ignores messages from other sessions", () => {
				const event = asEvent({
					type: "message.updated",
					properties: {
						info: {
							id: MSG_ID,
							sessionID: "other-session",
							role: "assistant",
							time: { created: 1000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});
		});

		// ── message.part.updated ────────────────────────────────────

		describe("message.part.updated", () => {
			beforeEach(() => {
				// Set up state as if we already received the assistant message
				state.assistantMessageId = MSG_ID;
			});

			it("emits start-step for step-start parts", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "step-start",
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 1);
				assert.strictEqual(chunks[0].type, "start-step");
			});

			it("emits finish-step for step-finish parts", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "step-finish",
							reason: "done",
							cost: 0.01,
							tokens: {
								input: 100,
								output: 50,
								reasoning: 0,
								cache: { read: 0, write: 0 },
							},
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 1);
				assert.strictEqual(chunks[0].type, "finish-step");
			});

			// ── text parts ──────────────────────────────────────────

			it("emits text-start for new text part (no time.end)", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-text-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "text",
							text: "",
							time: { start: 1000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 1);
				assert.strictEqual(chunks[0].type, "text-start");
				assert.strictEqual((chunks[0] as { id: string }).id, "part-text-1");
				assert.ok(state.startedParts.has("part-text-1"));
			});

			it("does not re-emit text-start for already-started text part", () => {
				state.startedParts.add("part-text-1");

				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-text-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "text",
							text: "partial text",
							time: { start: 1000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});

			it("emits text-end when text part completes (has time.end)", () => {
				state.startedParts.add("part-text-1");

				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-text-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "text",
							text: "final text",
							time: { start: 1000, end: 2000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 1);
				assert.strictEqual(chunks[0].type, "text-end");
			});

			it("emits full text block for completed part that was never started", () => {
				// Simulates a case where we missed the deltas and only see the completed part
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-text-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "text",
							text: "complete text",
							time: { start: 1000, end: 2000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 3);
				assert.strictEqual(chunks[0].type, "text-start");
				assert.strictEqual(chunks[1].type, "text-delta");
				assert.strictEqual(
					(chunks[1] as { delta: string }).delta,
					"complete text",
				);
				assert.strictEqual(chunks[2].type, "text-end");
			});

			it("emits text-start + text-end (no delta) for completed part with empty text", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-text-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "text",
							text: "",
							time: { start: 1000, end: 2000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 2);
				assert.strictEqual(chunks[0].type, "text-start");
				assert.strictEqual(chunks[1].type, "text-end");
			});

			// ── reasoning parts ─────────────────────────────────────

			it("emits reasoning-start for new reasoning part (no time.end)", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-reason-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "reasoning",
							text: "",
							time: { start: 1000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 1);
				assert.strictEqual(chunks[0].type, "reasoning-start");
				assert.ok(state.startedParts.has("part-reason-1"));
			});

			it("emits reasoning-end when reasoning part completes", () => {
				state.startedParts.add("part-reason-1");

				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-reason-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "reasoning",
							text: "thinking done",
							time: { start: 1000, end: 2000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 1);
				assert.strictEqual(chunks[0].type, "reasoning-end");
			});

			it("emits full reasoning block for completed part never started", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-reason-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "reasoning",
							text: "full reasoning",
							time: { start: 1000, end: 2000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 3);
				assert.strictEqual(chunks[0].type, "reasoning-start");
				assert.strictEqual(chunks[1].type, "reasoning-delta");
				assert.strictEqual(
					(chunks[1] as { delta: string }).delta,
					"full reasoning",
				);
				assert.strictEqual(chunks[2].type, "reasoning-end");
			});

			// ── tool parts ──────────────────────────────────────────

			it("emits tool-input-start for new tool part", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-tool-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "tool",
							callID: "call-1",
							tool: "Read",
							state: {
								status: "pending",
								input: { file_path: "/test.txt" },
								raw: "",
							},
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				const toolStart = chunks.find((c) => c.type === "tool-input-start");
				assert.ok(toolStart);
				assert.strictEqual(
					(toolStart as { toolCallId: string }).toolCallId,
					"call-1",
				);
				assert.strictEqual(
					(toolStart as { toolName: string }).toolName,
					"Read",
				);
				assert.ok(state.startedToolParts.has("part-tool-1"));
			});

			it("does not re-emit tool-input-start for already-started tool", () => {
				state.startedToolParts.add("part-tool-1");

				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-tool-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "tool",
							callID: "call-1",
							tool: "Read",
							state: {
								status: "running",
								input: { file_path: "/test.txt" },
								time: { start: 1000 },
							},
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				const toolStarts = chunks.filter((c) => c.type === "tool-input-start");
				assert.strictEqual(toolStarts.length, 0);
			});

			it("emits tool-input-available when tool has input", () => {
				state.startedToolParts.add("part-tool-1");

				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-tool-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "tool",
							callID: "call-1",
							tool: "Bash",
							state: {
								status: "running",
								input: { command: "ls -la" },
								time: { start: 1000 },
							},
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				const inputAvailable = chunks.find(
					(c) => c.type === "tool-input-available",
				);
				assert.ok(inputAvailable);
				assert.deepStrictEqual((inputAvailable as { input: unknown }).input, {
					command: "ls -la",
				});
			});

			it("emits tool-output-available when tool completes", () => {
				state.startedToolParts.add("part-tool-1");

				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-tool-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "tool",
							callID: "call-1",
							tool: "Read",
							state: {
								status: "completed",
								input: { file_path: "/test.txt" },
								output: "file contents here",
								title: "Read",
								metadata: {},
								time: { start: 1000, end: 2000 },
							},
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				const output = chunks.find((c) => c.type === "tool-output-available");
				assert.ok(output);
				assert.strictEqual(
					(output as { output: string }).output,
					"file contents here",
				);
			});

			it("emits tool-output-error when tool errors", () => {
				state.startedToolParts.add("part-tool-1");

				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-tool-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "tool",
							callID: "call-1",
							tool: "Bash",
							state: {
								status: "error",
								input: { command: "bad-cmd" },
								error: "command not found",
								time: { start: 1000, end: 2000 },
							},
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				const errChunk = chunks.find((c) => c.type === "tool-output-error");
				assert.ok(errChunk);
				assert.strictEqual(
					(errChunk as { errorText: string }).errorText,
					"command not found",
				);
			});

			it("emits full tool lifecycle: start + input + output", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-tool-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "tool",
							callID: "call-1",
							tool: "Read",
							state: {
								status: "completed",
								input: { file_path: "/x" },
								output: "ok",
								title: "Read",
								metadata: {},
								time: { start: 1000, end: 2000 },
							},
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);

				assert.strictEqual(chunks.length, 3);
				assert.strictEqual(chunks[0].type, "tool-input-start");
				assert.strictEqual(chunks[1].type, "tool-input-available");
				assert.strictEqual(chunks[2].type, "tool-output-available");
			});

			// ── file parts ──────────────────────────────────────────

			it("emits text block for file parts", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-file-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "file",
							mime: "image/png",
							filename: "screenshot.png",
							url: "https://example.com/screenshot.png",
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 3);
				assert.strictEqual(chunks[0].type, "text-start");
				assert.strictEqual(chunks[1].type, "text-delta");
				assert.strictEqual(
					(chunks[1] as { delta: string }).delta,
					"[File: screenshot.png]",
				);
				assert.strictEqual(chunks[2].type, "text-end");
			});

			it("falls back to URL when file has no filename", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-file-1",
							sessionID: SESSION_ID,
							messageID: MSG_ID,
							type: "file",
							mime: "text/plain",
							url: "https://example.com/data.txt",
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(
					(chunks[1] as { delta: string }).delta,
					"[File: https://example.com/data.txt]",
				);
			});

			// ── filtering ───────────────────────────────────────────

			it("ignores parts from other sessions", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-1",
							sessionID: "other-session",
							messageID: MSG_ID,
							type: "text",
							text: "should be ignored",
							time: { start: 1000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});

			it("ignores parts from user messages (different messageID)", () => {
				const event = asEvent({
					type: "message.part.updated",
					properties: {
						part: {
							id: "part-1",
							sessionID: SESSION_ID,
							messageID: USER_MSG_ID,
							type: "text",
							text: "user's text",
							time: { start: 1000 },
						},
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});

			it("ignores snapshot, patch, agent, retry, compaction parts", () => {
				for (const type of [
					"snapshot",
					"patch",
					"agent",
					"retry",
					"compaction",
				]) {
					const event = asEvent({
						type: "message.part.updated",
						properties: {
							part: {
								id: `part-${type}`,
								sessionID: SESSION_ID,
								messageID: MSG_ID,
								type,
							},
						},
					});

					const chunks = translateEventsToChunks(event, SESSION_ID, state);
					assert.strictEqual(chunks.length, 0, `Should ignore ${type} parts`);
				}
			});
		});

		// ── message.part.delta ───────────────────────────────────────

		describe("message.part.delta", () => {
			beforeEach(() => {
				state.assistantMessageId = MSG_ID;
			});

			it("emits text-delta for text field deltas", () => {
				state.startedParts.add("part-text-1");

				const event = asEvent({
					type: "message.part.delta",
					properties: {
						sessionID: SESSION_ID,
						messageID: MSG_ID,
						partID: "part-text-1",
						delta: "Hello ",
						field: "text",
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 1);
				assert.strictEqual(chunks[0].type, "text-delta");
				assert.strictEqual((chunks[0] as { delta: string }).delta, "Hello ");
			});

			it("auto-emits text-start before first delta if part not started", () => {
				// This tests the key case where deltas arrive before part-updated
				const event = asEvent({
					type: "message.part.delta",
					properties: {
						sessionID: SESSION_ID,
						messageID: MSG_ID,
						partID: "part-text-1",
						delta: "Hello",
						field: "text",
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 2);
				assert.strictEqual(chunks[0].type, "text-start");
				assert.strictEqual((chunks[0] as { id: string }).id, "part-text-1");
				assert.strictEqual(chunks[1].type, "text-delta");
				assert.ok(state.startedParts.has("part-text-1"));
			});

			it("emits reasoning-delta for known reasoning parts (field is 'text')", () => {
				// OpenCode sends field:"text" for reasoning deltas because
				// ReasoningPart stores content in its `text` property.
				// We rely on reasoningPartIds to route correctly.
				state.startedParts.add("part-reason-1");
				state.reasoningPartIds.add("part-reason-1");

				const event = asEvent({
					type: "message.part.delta",
					properties: {
						sessionID: SESSION_ID,
						messageID: MSG_ID,
						partID: "part-reason-1",
						delta: "thinking...",
						field: "text",
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 1);
				assert.strictEqual(chunks[0].type, "reasoning-delta");
			});

			it("auto-emits reasoning-start before first delta for known reasoning part", () => {
				// Part was registered as reasoning (via message.part.updated)
				// but not yet started
				state.reasoningPartIds.add("part-reason-1");

				const event = asEvent({
					type: "message.part.delta",
					properties: {
						sessionID: SESSION_ID,
						messageID: MSG_ID,
						partID: "part-reason-1",
						delta: "Let me think",
						field: "text",
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 2);
				assert.strictEqual(chunks[0].type, "reasoning-start");
				assert.strictEqual(chunks[1].type, "reasoning-delta");
				assert.ok(state.startedParts.has("part-reason-1"));
			});

			it("ignores deltas from other sessions", () => {
				const event = asEvent({
					type: "message.part.delta",
					properties: {
						sessionID: "other-session",
						messageID: MSG_ID,
						partID: "part-text-1",
						delta: "should be ignored",
						field: "text",
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});

			it("ignores deltas from user messages (different messageID)", () => {
				const event = asEvent({
					type: "message.part.delta",
					properties: {
						sessionID: SESSION_ID,
						messageID: USER_MSG_ID,
						partID: "part-text-1",
						delta: "user message delta",
						field: "text",
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});

			it("ignores deltas with empty delta string", () => {
				const event = asEvent({
					type: "message.part.delta",
					properties: {
						sessionID: SESSION_ID,
						messageID: MSG_ID,
						partID: "part-text-1",
						delta: "",
						field: "text",
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});

			it("ignores deltas with null/undefined delta", () => {
				const event = asEvent({
					type: "message.part.delta",
					properties: {
						sessionID: SESSION_ID,
						messageID: MSG_ID,
						partID: "part-text-1",
						delta: null,
						field: "text",
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});

			it("ignores deltas for unknown fields", () => {
				const event = asEvent({
					type: "message.part.delta",
					properties: {
						sessionID: SESSION_ID,
						messageID: MSG_ID,
						partID: "part-1",
						delta: "some delta",
						field: "unknown-field",
					},
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});
		});

		// ── unknown event types ─────────────────────────────────────

		describe("unknown events", () => {
			it("returns empty array for session.idle events", () => {
				const event = asEvent({
					type: "session.idle",
					properties: { sessionID: SESSION_ID },
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});

			it("returns empty array for session.status events", () => {
				const event = asEvent({
					type: "session.status",
					properties: { sessionID: SESSION_ID, status: "busy" },
				});

				const chunks = translateEventsToChunks(event, SESSION_ID, state);
				assert.strictEqual(chunks.length, 0);
			});
		});

		// ── full streaming scenario ─────────────────────────────────

		describe("end-to-end streaming scenarios", () => {
			it("handles a complete text response flow", () => {
				const allChunks: ReturnType<typeof translateEventsToChunks> = [];

				// 1. Assistant message created
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.updated",
							properties: {
								info: {
									id: MSG_ID,
									sessionID: SESSION_ID,
									role: "assistant",
									time: { created: 1000 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 2. Step starts
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "step-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "step-start",
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 3. Text part created (no time.end)
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "text-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "text",
									text: "",
									time: { start: 1000 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 4. Text deltas stream in
				for (const word of ["Hello", " ", "world", "!"]) {
					allChunks.push(
						...translateEventsToChunks(
							asEvent({
								type: "message.part.delta",
								properties: {
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									partID: "text-1",
									delta: word,
									field: "text",
								},
							}),
							SESSION_ID,
							state,
						),
					);
				}

				// 5. Text part completed
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "text-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "text",
									text: "Hello world!",
									time: { start: 1000, end: 2000 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 6. Step finishes
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "step-finish-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "step-finish",
									reason: "done",
									cost: 0.01,
									tokens: {
										input: 100,
										output: 50,
										reasoning: 0,
										cache: { read: 0, write: 0 },
									},
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 7. Message completed
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.updated",
							properties: {
								info: {
									id: MSG_ID,
									sessionID: SESSION_ID,
									role: "assistant",
									time: { created: 1000, completed: 3000 },
									finish: "stop",
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// Verify the full sequence
				const types = allChunks.map((c) => c.type);
				assert.deepStrictEqual(types, [
					"start", // message created
					"start-step", // step starts
					"text-start", // text part created
					"text-delta", // "Hello"
					"text-delta", // " "
					"text-delta", // "world"
					"text-delta", // "!"
					"text-end", // text part completed
					"finish-step", // step finishes
					"finish", // message completed
				]);
			});

			it("handles deltas arriving before part-updated (out-of-order)", () => {
				const allChunks: ReturnType<typeof translateEventsToChunks> = [];

				// 1. Message created
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.updated",
							properties: {
								info: {
									id: MSG_ID,
									sessionID: SESSION_ID,
									role: "assistant",
									time: { created: 1000 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 2. Delta arrives BEFORE part-updated (this happens in practice)
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.delta",
							properties: {
								sessionID: SESSION_ID,
								messageID: MSG_ID,
								partID: "text-1",
								delta: "First chunk",
								field: "text",
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 3. Part-updated arrives (text already started via delta)
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "text-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "text",
									text: "",
									time: { start: 1000 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 4. More deltas
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.delta",
							properties: {
								sessionID: SESSION_ID,
								messageID: MSG_ID,
								partID: "text-1",
								delta: " more text",
								field: "text",
							},
						}),
						SESSION_ID,
						state,
					),
				);

				const types = allChunks.map((c) => c.type);
				assert.deepStrictEqual(types, [
					"start", // message created
					"text-start", // auto-emitted before first delta
					"text-delta", // "First chunk"
					// part-updated arrives but text-start already emitted → no-op
					"text-delta", // " more text"
				]);
			});

			it("handles tool use flow", () => {
				const allChunks: ReturnType<typeof translateEventsToChunks> = [];
				state.assistantMessageId = MSG_ID;

				// 1. Tool part pending
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "tool-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "tool",
									callID: "call-1",
									tool: "Read",
									state: {
										status: "pending",
										input: { file_path: "/test.txt" },
										raw: "",
									},
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 2. Tool running
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "tool-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "tool",
									callID: "call-1",
									tool: "Read",
									state: {
										status: "running",
										input: { file_path: "/test.txt" },
										time: { start: 1000 },
									},
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 3. Tool completed
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "tool-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "tool",
									callID: "call-1",
									tool: "Read",
									state: {
										status: "completed",
										input: { file_path: "/test.txt" },
										output: "contents",
										title: "Read",
										metadata: {},
										time: { start: 1000, end: 2000 },
									},
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				const types = allChunks.map((c) => c.type);
				assert.deepStrictEqual(types, [
					"tool-input-start", // first time seeing tool
					"tool-input-available", // pending has input
					"tool-input-available", // running has input (re-sent)
					"tool-input-available", // completed has input
					"tool-output-available", // completed has output
				]);
			});

			it("filters out user message events in a mixed flow", () => {
				const allChunks: ReturnType<typeof translateEventsToChunks> = [];

				// 1. User message.updated — should be ignored
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.updated",
							properties: {
								info: {
									id: USER_MSG_ID,
									sessionID: SESSION_ID,
									role: "user",
									time: { created: 1000 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 2. User message part.updated — should be ignored (no assistant ID set yet)
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "user-part-1",
									sessionID: SESSION_ID,
									messageID: USER_MSG_ID,
									type: "text",
									text: "User question",
									time: { start: 1000, end: 1000 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 3. User message delta — should be ignored
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.delta",
							properties: {
								sessionID: SESSION_ID,
								messageID: USER_MSG_ID,
								partID: "user-part-1",
								delta: "user text",
								field: "text",
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 4. Now assistant message comes
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.updated",
							properties: {
								info: {
									id: MSG_ID,
									sessionID: SESSION_ID,
									role: "assistant",
									time: { created: 2000 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 5. Assistant delta — should be processed
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.delta",
							properties: {
								sessionID: SESSION_ID,
								messageID: MSG_ID,
								partID: "assistant-text-1",
								delta: "Hello!",
								field: "text",
							},
						}),
						SESSION_ID,
						state,
					),
				);

				const types = allChunks.map((c) => c.type);
				assert.deepStrictEqual(types, [
					"start", // assistant message
					"text-start", // auto-start from first delta
					"text-delta", // "Hello!"
				]);
			});

			it("handles reasoning followed by text in streaming", () => {
				const allChunks: ReturnType<typeof translateEventsToChunks> = [];
				state.assistantMessageId = MSG_ID;

				// 1. Reasoning part created (registers in reasoningPartIds)
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "reason-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "reasoning",
									text: "",
									time: { start: 1000 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 2. Reasoning deltas (field is "text" — OpenCode uses the property name)
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.delta",
							properties: {
								sessionID: SESSION_ID,
								messageID: MSG_ID,
								partID: "reason-1",
								delta: "Let me think",
								field: "text",
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 3. More reasoning
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.delta",
							properties: {
								sessionID: SESSION_ID,
								messageID: MSG_ID,
								partID: "reason-1",
								delta: " about this...",
								field: "text",
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 4. Reasoning completes
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.updated",
							properties: {
								part: {
									id: "reason-1",
									sessionID: SESSION_ID,
									messageID: MSG_ID,
									type: "reasoning",
									text: "Let me think about this...",
									time: { start: 1000, end: 1500 },
								},
							},
						}),
						SESSION_ID,
						state,
					),
				);

				// 5. Text delta starts
				allChunks.push(
					...translateEventsToChunks(
						asEvent({
							type: "message.part.delta",
							properties: {
								sessionID: SESSION_ID,
								messageID: MSG_ID,
								partID: "text-1",
								delta: "Here is my answer",
								field: "text",
							},
						}),
						SESSION_ID,
						state,
					),
				);

				const types = allChunks.map((c) => c.type);
				assert.deepStrictEqual(types, [
					"reasoning-start", // part created
					"reasoning-delta", // "Let me think"
					"reasoning-delta", // " about this..."
					"reasoning-end", // reasoning completes
					"text-start", // auto-start from first text delta
					"text-delta", // "Here is my answer"
				]);
			});
		});
	});
});
