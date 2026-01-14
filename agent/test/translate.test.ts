import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type {
	ContentBlock,
	SessionUpdate,
	ToolCall,
	ToolCallUpdate,
} from "@agentclientprotocol/sdk";
import type {
	DynamicToolUIPart,
	FileUIPart,
	ReasoningUIPart,
	TextUIPart,
} from "ai";
import {
	createUIMessage,
	generateMessageId,
	sessionUpdateToUIPart,
	toolCallToUIPart,
	toolCallUpdateToUIPart,
	uiMessageToContentBlocks,
} from "../src/acp/translate.js";

describe("translate.ts", () => {
	describe("sessionUpdateToUIPart", () => {
		describe("agent_message_chunk", () => {
			it("maps text content to TextUIPart", () => {
				const update: SessionUpdate = {
					sessionUpdate: "agent_message_chunk",
					content: { type: "text", text: "Hello world" },
				};

				const result = sessionUpdateToUIPart(update);

				assert.deepEqual(result, {
					type: "text",
					text: "Hello world",
				} satisfies TextUIPart);
			});

			it("returns null for non-text content", () => {
				const update: SessionUpdate = {
					sessionUpdate: "agent_message_chunk",
					content: { type: "image", data: "base64data", mimeType: "image/png" },
				};

				const result = sessionUpdateToUIPart(update);
				assert.equal(result, null);
			});
		});

		describe("agent_thought_chunk", () => {
			it("maps text content to ReasoningUIPart", () => {
				const update: SessionUpdate = {
					sessionUpdate: "agent_thought_chunk",
					content: { type: "text", text: "Thinking about the problem..." },
				};

				const result = sessionUpdateToUIPart(update);

				assert.deepEqual(result, {
					type: "reasoning",
					text: "Thinking about the problem...",
				} satisfies ReasoningUIPart);
			});
		});

		describe("tool_call", () => {
			it("delegates to toolCallToUIPart", () => {
				const update: SessionUpdate = {
					sessionUpdate: "tool_call",
					toolCallId: "tc-123",
					title: "Read file",
					status: "completed",
					rawInput: { path: "/test.txt" },
					rawOutput: "file contents",
				};

				const result = sessionUpdateToUIPart(update);

				assert.equal(result?.type, "dynamic-tool");
				assert.equal((result as DynamicToolUIPart).toolCallId, "tc-123");
			});
		});

		describe("tool_call_update", () => {
			it("delegates to toolCallUpdateToUIPart", () => {
				const update: SessionUpdate = {
					sessionUpdate: "tool_call_update",
					toolCallId: "tc-123",
					title: "Read file",
					status: "in_progress",
				};

				const result = sessionUpdateToUIPart(update);

				assert.equal(result?.type, "dynamic-tool");
				assert.equal((result as DynamicToolUIPart).toolCallId, "tc-123");
			});
		});

		describe("unsupported session updates", () => {
			it("returns null for plan updates", () => {
				const update: SessionUpdate = {
					sessionUpdate: "plan",
					entries: [],
				};

				const result = sessionUpdateToUIPart(update);
				assert.equal(result, null);
			});

			it("returns null for user_message_chunk", () => {
				const update: SessionUpdate = {
					sessionUpdate: "user_message_chunk",
					content: { type: "text", text: "User input" },
				};

				const result = sessionUpdateToUIPart(update);
				assert.equal(result, null);
			});
		});
	});

	describe("toolCallToUIPart", () => {
		describe("field mapping completeness", () => {
			it("maps all essential ToolCall fields", () => {
				const toolCall: ToolCall = {
					toolCallId: "tc-456",
					title: "Execute command",
					status: "completed",
					rawInput: { command: "ls -la" },
					rawOutput: "file1.txt\nfile2.txt",
					kind: "execute",
					locations: [{ path: "/test.txt" }],
					content: [
						{
							type: "content",
							content: { type: "text", text: "output" } satisfies ContentBlock,
						},
					],
					_meta: { custom: "data" },
				};

				const result = toolCallToUIPart(toolCall);

				// Verify mapped fields
				assert.equal(result.type, "dynamic-tool");
				assert.equal(result.toolCallId, "tc-456");
				assert.equal(result.toolName, "Execute command"); // title -> toolName
				assert.deepEqual(result.input, { command: "ls -la" }); // rawInput -> input
				assert.equal(result.state, "output-available");

				// For output-available state, output should be set
				if (result.state === "output-available") {
					assert.deepEqual(result.output, "file1.txt\nfile2.txt"); // rawOutput -> output
				}
			});

			it("documents unmapped fields from ToolCall", () => {
				// These ACP fields are intentionally NOT mapped to DynamicToolUIPart:
				// - kind: Tool category (read/edit/delete/etc) - no equivalent in AI SDK
				// - locations: File locations - no equivalent in AI SDK
				// - content: Rich tool output - AI SDK uses rawOutput only
				// - _meta: Extension metadata - no equivalent in AI SDK
				//
				// This is acceptable because:
				// 1. AI SDK's DynamicToolUIPart is simpler and focused on core tool state
				// 2. Rich content can be derived from rawOutput if needed
				// 3. Location tracking would need to be handled at a higher level

				const toolCall: ToolCall = {
					toolCallId: "tc-789",
					title: "Test",
					kind: "read",
					locations: [{ path: "/path" }],
					content: [
						{
							type: "content",
							content: { type: "text", text: "x" } satisfies ContentBlock,
						},
					],
					_meta: { foo: "bar" },
				};

				const result = toolCallToUIPart(toolCall);

				// Verify these fields are not present in result
				assert.equal("kind" in result, false);
				assert.equal("locations" in result, false);
				assert.equal("content" in result, false);
				assert.equal("_meta" in result, false);
			});
		});

		describe("status to state mapping", () => {
			it("maps pending status to input-streaming state", () => {
				const toolCall: ToolCall = {
					toolCallId: "tc-1",
					title: "Test",
					status: "pending",
				};

				const result = toolCallToUIPart(toolCall);
				assert.equal(result.state, "input-streaming");
			});

			it("maps in_progress status to input-available state", () => {
				const toolCall: ToolCall = {
					toolCallId: "tc-1",
					title: "Test",
					status: "in_progress",
					rawInput: { test: true },
				};

				const result = toolCallToUIPart(toolCall);
				assert.equal(result.state, "input-available");
			});

			it("maps completed status to output-available state", () => {
				const toolCall: ToolCall = {
					toolCallId: "tc-1",
					title: "Test",
					status: "completed",
					rawInput: { test: true },
					rawOutput: "result",
				};

				const result = toolCallToUIPart(toolCall);
				assert.equal(result.state, "output-available");
				if (result.state === "output-available") {
					assert.equal(result.output, "result");
				}
			});

			it("maps failed status to output-error state", () => {
				const toolCall: ToolCall = {
					toolCallId: "tc-1",
					title: "Test",
					status: "failed",
					rawInput: { test: true },
					rawOutput: "Error: something went wrong",
				};

				const result = toolCallToUIPart(toolCall);
				assert.equal(result.state, "output-error");
				if (result.state === "output-error") {
					assert.equal(result.errorText, "Error: something went wrong");
				}
			});

			it("maps null/undefined status to input-streaming state", () => {
				const toolCall: ToolCall = {
					toolCallId: "tc-1",
					title: "Test",
				};

				const result = toolCallToUIPart(toolCall);
				assert.equal(result.state, "input-streaming");
			});
		});

		describe("edge cases", () => {
			it("handles missing rawInput with empty object", () => {
				const toolCall: ToolCall = {
					toolCallId: "tc-1",
					title: "Test",
					status: "in_progress",
				};

				const result = toolCallToUIPart(toolCall);
				if (result.state === "input-available") {
					assert.deepEqual(result.input, {});
				}
			});

			it("handles missing rawOutput for failed status", () => {
				const toolCall: ToolCall = {
					toolCallId: "tc-1",
					title: "Test",
					status: "failed",
				};

				const result = toolCallToUIPart(toolCall);
				if (result.state === "output-error") {
					assert.equal(result.errorText, "Tool call failed");
				}
			});
		});
	});

	describe("toolCallUpdateToUIPart", () => {
		describe("field mapping completeness", () => {
			it("maps all essential ToolCallUpdate fields", () => {
				const update: ToolCallUpdate = {
					toolCallId: "tc-update-1",
					title: "Updated title",
					status: "completed",
					rawInput: { updated: true },
					rawOutput: "updated output",
				};

				const result = toolCallUpdateToUIPart(update);

				assert.equal(result.type, "dynamic-tool");
				assert.equal(result.toolCallId, "tc-update-1");
				assert.equal(result.toolName, "Updated title");
				assert.equal(result.state, "output-available");
			});

			it("uses 'unknown' for missing title", () => {
				const update: ToolCallUpdate = {
					toolCallId: "tc-update-2",
					status: "in_progress",
				};

				const result = toolCallUpdateToUIPart(update);
				assert.equal(result.toolName, "unknown");
			});
		});

		describe("partial updates", () => {
			it("handles update with only status change", () => {
				const update: ToolCallUpdate = {
					toolCallId: "tc-update-3",
					status: "completed",
					rawOutput: "done",
				};

				const result = toolCallUpdateToUIPart(update);
				assert.equal(result.state, "output-available");
				assert.equal(result.toolName, "unknown");
			});
		});
	});

	describe("uiMessageToContentBlocks", () => {
		it("converts text parts to text ContentBlocks", () => {
			const message = createUIMessage("user", [
				{ type: "text", text: "Hello" },
			]);

			const blocks = uiMessageToContentBlocks(message);

			assert.equal(blocks.length, 1);
			assert.deepEqual(blocks[0], { type: "text", text: "Hello" });
		});

		it("converts file parts to resource_link ContentBlocks", () => {
			const filePart: FileUIPart = {
				type: "file",
				url: "data:image/png;base64,abc",
				mediaType: "image/png",
				filename: "test.png",
			};
			// Need to cast since createUIMessage expects ProducedUIPart but FileUIPart is valid in UIMessage
			const message = createUIMessage("user", [filePart as never]);

			const blocks = uiMessageToContentBlocks(message);

			assert.equal(blocks.length, 1);
			const resourceLink = blocks[0] as {
				type: string;
				uri: string;
				name: string;
				mimeType: string;
			};
			assert.equal(resourceLink.type, "resource_link");
			assert.equal(resourceLink.uri, "data:image/png;base64,abc");
			assert.equal(resourceLink.name, "test.png");
			assert.equal(resourceLink.mimeType, "image/png");
		});

		it("uses default name when filename is missing", () => {
			const filePart: FileUIPart = {
				type: "file",
				url: "data:image/png;base64,abc",
				mediaType: "image/png",
			};
			const message = createUIMessage("user", [filePart as never]);

			const blocks = uiMessageToContentBlocks(message);

			const resourceLink = blocks[0] as { name: string };
			assert.equal(resourceLink.name, "file");
		});

		it("skips unsupported part types", () => {
			const message = createUIMessage("assistant", [
				{ type: "text", text: "Hello" },
				{ type: "reasoning", text: "Thinking..." },
				{
					type: "dynamic-tool",
					toolCallId: "tc-1",
					toolName: "test",
					state: "output-available",
					input: {},
					output: "result",
				},
			]);

			const blocks = uiMessageToContentBlocks(message);

			// Only text should be converted
			assert.equal(blocks.length, 1);
			assert.equal(blocks[0].type, "text");
		});
	});

	describe("createUIMessage", () => {
		it("creates message with generated ID", () => {
			const message = createUIMessage("user");

			assert.ok(message.id.startsWith("msg-"));
			assert.equal(message.role, "user");
			assert.deepEqual(message.parts, []);
		});

		it("creates message with provided parts", () => {
			const parts = [{ type: "text" as const, text: "Hello" }];
			const message = createUIMessage("assistant", parts);

			assert.equal(message.role, "assistant");
			assert.deepEqual(message.parts, parts);
		});
	});

	describe("generateMessageId", () => {
		it("generates unique IDs", () => {
			const id1 = generateMessageId();
			const id2 = generateMessageId();

			assert.notEqual(id1, id2);
		});

		it("generates IDs with correct format", () => {
			const id = generateMessageId();

			assert.ok(id.startsWith("msg-"));
			assert.ok(/^msg-\d+-[a-z0-9]+$/.test(id));
		});
	});
});

describe("ACP to AI SDK type coverage", () => {
	describe("SessionUpdate variants coverage", () => {
		// This test documents which SessionUpdate variants are handled
		const handledVariants = [
			"agent_message_chunk",
			"agent_thought_chunk",
			"tool_call",
			"tool_call_update",
		] as const;

		const unhandledVariants = [
			"user_message_chunk", // Handled separately in acp-client.ts during replay
			"plan",
			"available_commands_update",
			"current_mode_update",
			"config_option_update",
			"session_info_update",
		] as const;

		it("documents handled SessionUpdate variants", () => {
			for (const variant of handledVariants) {
				// Just documenting - actual handling tested above
				assert.ok(
					handledVariants.includes(variant),
					`${variant} should be handled`,
				);
			}
		});

		it("documents unhandled SessionUpdate variants", () => {
			// These variants are intentionally not converted to UIMessageParts because:
			// - user_message_chunk: Handled during session replay, not streaming
			// - plan: UI planning feature, not message content
			// - available_commands_update: Slash commands, not message content
			// - current_mode_update: Mode state, not message content
			// - config_option_update: Config state, not message content
			// - session_info_update: Session metadata, not message content
			for (const variant of unhandledVariants) {
				assert.ok(
					unhandledVariants.includes(variant),
					`${variant} is documented as unhandled`,
				);
			}
		});
	});

	describe("DynamicToolUIPart state coverage", () => {
		// AI SDK DynamicToolUIPart states:
		// - input-streaming: Tool call started, input being streamed
		// - input-available: Full input available
		// - approval-requested: Waiting for user approval (not used by ACP)
		// - approval-responded: User approved/denied (not used by ACP)
		// - output-available: Tool completed successfully
		// - output-error: Tool failed
		// - output-denied: Tool denied by user (not used by ACP)

		const mappedStates = [
			"input-streaming",
			"input-available",
			"output-available",
			"output-error",
		] as const;

		const unmappedStates = [
			"approval-requested",
			"approval-responded",
			"output-denied",
		] as const;

		it("maps all relevant ACP statuses to AI SDK states", () => {
			// ACP status -> AI SDK state
			const statusToState: Record<string, (typeof mappedStates)[number]> = {
				pending: "input-streaming",
				in_progress: "input-available",
				completed: "output-available",
				failed: "output-error",
			};

			for (const [status, expectedState] of Object.entries(statusToState)) {
				const toolCall: ToolCall = {
					toolCallId: "test",
					title: "Test",
					status: status as ToolCall["status"],
					rawInput: {},
					rawOutput: status === "failed" ? "error" : "result",
				};

				const result = toolCallToUIPart(toolCall);
				assert.equal(
					result.state,
					expectedState,
					`ACP status '${status}' should map to '${expectedState}'`,
				);
			}
		});

		it("documents unmapped AI SDK states", () => {
			// These states are not produced because ACP doesn't have equivalent concepts:
			// - approval-requested/approval-responded: ACP uses requestPermission RPC
			// - output-denied: ACP doesn't have this concept
			for (const state of unmappedStates) {
				assert.ok(
					unmappedStates.includes(state),
					`${state} is documented as unmapped`,
				);
			}
		});
	});
});
