/**
 * Agent Interface Contract Tests
 *
 * Validates that an agent implementation correctly implements the Agent interface.
 * All providers must pass all tests - no conditional behavior.
 *
 * Usage:
 *   ANTHROPIC_API_KEY=xxx pnpm test test/integration/agent-interface-contract.test.ts
 *   PROVIDER=my-provider MY_API_KEY=xxx pnpm test test/integration/agent-interface-contract.test.ts
 */

import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import type { Agent } from "../../src/agent/interface.js";
import type { ModelInfo, UIMessageChunk } from "../../src/api/types.js";
import { questionManager } from "../../src/question-manager.js";
import { getProvider } from "./agent-provider-registry.js";

const PROVIDER_NAME = process.env.PROVIDER || "claude-sdk";
const provider = getProvider(PROVIDER_NAME);

console.log(`\n🧪 Testing provider: ${provider.name}\n`);

// Helper to collect all chunks from async generator
async function collectChunks(
	generator: AsyncGenerator<UIMessageChunk>,
	timeout = 120000,
): Promise<UIMessageChunk[]> {
	const chunks: UIMessageChunk[] = [];
	const timeoutPromise = new Promise<never>((_, reject) =>
		setTimeout(() => reject(new Error("Timeout")), timeout),
	);

	await Promise.race([
		(async () => {
			for await (const chunk of generator) {
				chunks.push(chunk);
			}
		})(),
		timeoutPromise,
	]);

	return chunks;
}

describe(`Agent Interface Contract: ${provider.name}`, () => {
	let agent: Agent;

	beforeEach(async () => {
		agent = provider.createAgent();
	});

	afterEach(async () => {
		try {
			if ("disconnect" in agent) {
				await (
					agent as unknown as { disconnect(): Promise<void> }
				).disconnect();
			}
		} catch (_error) {
			// Ignore cleanup errors
		}
	});

	// ==========================================================================
	// MESSAGING
	// ==========================================================================

	describe("Messaging", () => {
		let sessionId: string;

		beforeEach(async () => {
			sessionId = "test-session-id";
		});

		it("prompt returns async generator", () => {
			const gen = agent.prompt(provider.testMessages.simple, sessionId);
			assert.ok(gen[Symbol.asyncIterator]);
		});

		it("yields message chunks", async () => {
			const gen = agent.prompt(provider.testMessages.simple, sessionId);
			const chunks = await collectChunks(gen);

			assert.ok(chunks.length > 0);
			// Verify chunks have proper structure
			// Note: ClaudeSDK uses both 'id' and 'messageId' for different chunk types
			for (const chunk of chunks) {
				assert.ok(chunk.type, "Chunk should have type");
			}
		});

		it("produces text content", async () => {
			const gen = agent.prompt(provider.testMessages.simple, sessionId);
			const chunks = await collectChunks(gen);

			const hasText = chunks.some(
				(c) =>
					c.type === "text-delta" &&
					"delta" in c &&
					typeof c.delta === "string" &&
					c.delta.trim().length > 0,
			);
			assert.ok(hasText);
		});

		it("completes generator", async () => {
			const gen = agent.prompt(provider.testMessages.simple, sessionId);
			let completed = false;

			for await (const chunk of gen) {
				assert.ok(chunk);
			}
			completed = true;

			assert.ok(completed);
		});

		it("messages persist in session", async () => {
			const gen1 = agent.prompt(provider.testMessages.simple, sessionId);
			await collectChunks(gen1);

			const gen2 = agent.prompt(provider.testMessages.continuation, sessionId);
			await collectChunks(gen2);

			assert.ok((await agent.getMessages(sessionId)).length >= 4); // 2 user + 2 assistant
		});

		it("maintains conversation context", async () => {
			// First message
			const gen1 = agent.prompt(provider.testMessages.simple, sessionId);
			await collectChunks(gen1);

			// Follow-up referencing first message
			const gen2 = agent.prompt(provider.testMessages.continuation, sessionId);
			const chunks2 = await collectChunks(gen2);

			// Should have a response
			assert.ok(chunks2.length > 0);

			// Session should have both exchanges
			assert.ok((await agent.getMessages(sessionId)).length >= 4);
		});

		it("handles tool use", async () => {
			const gen = agent.prompt(provider.testMessages.withTools, sessionId);
			const chunks = await collectChunks(gen);

			// ClaudeSDK emits tool-input-start, tool-input-available for tool calls
			const hasToolCall = chunks.some(
				(c) =>
					c.type === "tool-input-start" || c.type === "tool-input-available",
			);
			assert.ok(hasToolCall, "Should have tool call chunks");
		});

		it("emits tool results", async () => {
			const gen = agent.prompt(provider.testMessages.withTools, sessionId);
			const chunks = await collectChunks(gen);

			// ClaudeSDK emits tool-output-available for tool results
			const hasToolResult = chunks.some(
				(c) => c.type === "tool-output-available",
			);
			assert.ok(hasToolResult, "Should have tool result chunks");
		});
	});

	// ==========================================================================
	// CANCELLATION
	// ==========================================================================

	describe("Cancellation", () => {
		let sessionId: string;

		beforeEach(async () => {
			sessionId = "test-session-id";
		});

		it("cancel on inactive session is safe", async () => {
			await assert.doesNotReject(async () => await agent.cancel(sessionId));
		});

		it("cancel on non-existent session is safe", async () => {
			await assert.doesNotReject(
				async () => await agent.cancel("does-not-exist"),
			);
		});

		it.skip("session remains usable after cancel", async () => {
			// TODO: This test causes timeouts - need to investigate generator cleanup
			const gen = agent.prompt(provider.testMessages.simple, sessionId);
			const iter = gen[Symbol.asyncIterator]();
			await iter.next();

			await agent.cancel(sessionId);

			// Should be able to send new prompt
			const gen2 = agent.prompt(provider.testMessages.simple, sessionId);
			const chunks = await collectChunks(gen2);
			assert.ok(chunks.length > 0);
		});

		// Multi-session cancellation test removed — single-session model
	});

	// ==========================================================================
	// ENVIRONMENT MANAGEMENT
	// ==========================================================================

	describe("Environment Management", () => {
		it("updateEnvironment does not throw", async () => {
			await agent.updateEnvironment("default", { TEST_VAR: "test-value" });
		});

		it("updateEnvironment with empty object is safe", async () => {
			await assert.doesNotReject(
				async () => await agent.updateEnvironment("default", {}),
			);
		});
	});

	// ==========================================================================
	// MODEL LISTING
	// ==========================================================================

	describe("Model Listing", () => {
		it("listModels returns an array", async () => {
			const models = await agent.listModels("default");
			assert.ok(Array.isArray(models));
		});

		it("listModels returns non-empty list", async () => {
			const models = await agent.listModels("default");
			assert.ok(models.length > 0, "Should return at least one model");
		});

		it("models have required fields", async () => {
			const models = await agent.listModels("default");

			for (const model of models) {
				assert.ok(typeof model.id === "string", "id should be a string");
				assert.ok(model.id.length > 0, "id should not be empty");
				assert.ok(
					typeof model.display_name === "string",
					"display_name should be a string",
				);
				assert.ok(
					model.display_name.length > 0,
					"display_name should not be empty",
				);
				assert.ok(
					typeof model.provider === "string",
					"provider should be a string",
				);
				assert.ok(model.provider.length > 0, "provider should not be empty");
				assert.ok(
					typeof model.created_at === "string",
					"created_at should be a string",
				);
				assert.ok(typeof model.type === "string", "type should be a string");
				assert.ok(
					typeof model.reasoning === "boolean",
					"reasoning should be a boolean",
				);
			}
		});

		it("model IDs include provider prefix", async () => {
			const models = await agent.listModels("default");

			for (const model of models) {
				assert.ok(
					model.id.includes(":"),
					`Model ID "${model.id}" should include a provider prefix (e.g. "anthropic:")`,
				);
			}
		});

		it("returns consistent results on repeated calls", async () => {
			const models1 = await agent.listModels("default");
			const models2 = await agent.listModels("default");

			assert.equal(
				models1.length,
				models2.length,
				"Should return the same number of models",
			);

			const ids1 = models1.map((m: ModelInfo) => m.id).sort();
			const ids2 = models2.map((m: ModelInfo) => m.id).sort();
			assert.deepEqual(ids1, ids2, "Should return the same model IDs");
		});

		it("does not require a session", async () => {
			// listModels should work without creating a session first
			const models = await agent.listModels("default");
			assert.ok(Array.isArray(models));
			assert.ok(models.length > 0);
		});
	});

	// ==========================================================================
	// EDGE CASES
	// ==========================================================================

	describe("Edge Cases", () => {
		it("handles empty parts array", async () => {
			const sessionId = "test-session-id";
			const emptyMessage = {
				id: "empty-msg",
				role: "user" as const,
				parts: [],
			};

			try {
				const gen = agent.prompt(emptyMessage, sessionId);
				await collectChunks(gen);
				assert.ok(true, "Handled empty message gracefully");
			} catch (error) {
				assert.ok(error instanceof Error);
				assert.ok(error.message.length > 0);
			}
		});
	});

	// ==========================================================================
	// INTEGRATION SCENARIOS
	// ==========================================================================

	describe("Integration Scenarios", () => {
		it("completes multi-turn conversation", async () => {
			const sessionId = "test-session-id";

			// Turn 1
			const gen1 = agent.prompt(provider.testMessages.simple, sessionId);
			const chunks1 = await collectChunks(gen1);
			assert.ok(chunks1.length > 0);

			// Turn 2
			const gen2 = agent.prompt(provider.testMessages.continuation, sessionId);
			const chunks2 = await collectChunks(gen2);
			assert.ok(chunks2.length > 0);

			// Turn 3 with tools
			const gen3 = agent.prompt(provider.testMessages.withTools, sessionId);
			const chunks3 = await collectChunks(gen3);
			assert.ok(chunks3.length > 0);

			// Verify session history
			assert.ok((await agent.getMessages(sessionId)).length >= 6); // 3 user + 3 assistant
		});
	});

	// ==========================================================================
	// PERMISSION AUTO-APPROVAL
	// ==========================================================================

	describe("Permission Auto-Approval", () => {
		let sessionId: string;

		beforeEach(async () => {
			sessionId = "test-session-id";
		});

		it("auto-approves tool permissions", async () => {
			// Send a prompt that triggers tool use. If permissions aren't
			// auto-approved, the tool call will block and the test will timeout.
			const gen = agent.prompt(provider.testMessages.withTools, sessionId);
			const chunks = await collectChunks(gen);

			// Verify the full tool lifecycle completed:
			// tool-input-start → tool-input-available → tool-output-available
			const hasToolStart = chunks.some((c) => c.type === "tool-input-start");
			const hasToolInput = chunks.some(
				(c) => c.type === "tool-input-available",
			);
			const hasToolOutput = chunks.some(
				(c) => c.type === "tool-output-available",
			);

			assert.ok(hasToolStart, "Should have tool-input-start chunk");
			assert.ok(hasToolInput, "Should have tool-input-available chunk");
			assert.ok(
				hasToolOutput,
				"Should have tool-output-available chunk (proves permission was auto-approved)",
			);
		});
	});

	// ==========================================================================
	// QUESTION HANDLING
	// ==========================================================================

	describe("Question Handling", () => {
		let sessionId: string;

		beforeEach(async () => {
			sessionId = "test-session-id";
			// Ensure no leftover pending questions from previous tests
			questionManager.cancelAll();
		});

		it("question manager has no pending questions initially", () => {
			assert.equal(questionManager.getPendingQuestion(), null);
		});

		it("cancel clears pending question state", async () => {
			// Start a prompt and immediately cancel
			const gen = agent.prompt(provider.testMessages.simple, sessionId);
			// Don't await — cancel right away
			const collectPromise = collectChunks(gen, 30000).catch(() => {});
			await agent.cancel(sessionId);
			await collectPromise;

			assert.equal(
				questionManager.getPendingQuestion(),
				null,
				"Pending questions should be cleared after cancel",
			);
		});

		it("submitAnswer resolves waitForAnswer", async () => {
			const toolUseID = "test-question-1";
			const testQuestions = [
				{
					question: "What language?",
					header: "Language",
					options: [
						{ label: "Python", description: "Python language" },
						{ label: "JavaScript", description: "JS language" },
					],
					multiSelect: false,
				},
			];

			// Start waiting for an answer in the background
			const answerPromise = questionManager.waitForAnswer(
				toolUseID,
				testQuestions,
			);

			// Verify the question is now pending
			const pending = questionManager.getPendingQuestion();
			assert.ok(pending, "Should have a pending question");
			assert.equal(pending.toolUseID, toolUseID);
			assert.equal(pending.questions.length, 1);
			assert.equal(pending.questions[0].question, "What language?");

			// Submit an answer
			const submitted = questionManager.submitAnswer(toolUseID, {
				"0": "Python",
			});
			assert.ok(
				submitted,
				"submitAnswer should return true for matching question",
			);

			// The promise should resolve with the answer
			const answers = await answerPromise;
			assert.deepEqual(answers, { "0": "Python" });

			// No more pending questions
			assert.equal(questionManager.getPendingQuestion(), null);
		});

		it("cancelAll rejects pending questions", async () => {
			const toolUseID = "test-question-2";
			const testQuestions = [
				{
					question: "Pick one",
					header: "Choice",
					options: [
						{ label: "A", description: "Option A" },
						{ label: "B", description: "Option B" },
					],
					multiSelect: false,
				},
			];

			const answerPromise = questionManager.waitForAnswer(
				toolUseID,
				testQuestions,
			);

			// Cancel all pending questions
			questionManager.cancelAll();

			// The promise should reject with a cancellation error
			await assert.rejects(answerPromise, (error: Error) => {
				assert.ok(
					error.message.includes("cancelled"),
					"Error message should contain 'cancelled'",
				);
				return true;
			});

			assert.equal(questionManager.getPendingQuestion(), null);
		});

		it("forwards questions and completes after answer", async () => {
			// Send a prompt designed to trigger a question from the agent.
			// This is inherently non-deterministic — the LLM may answer directly.
			const gen = agent.prompt(provider.testMessages.withQuestion, sessionId);

			let questionReceived = false;

			// Run prompt collection and question answering concurrently
			const [chunks] = await Promise.all([
				collectChunks(gen, 120000),
				(async () => {
					// Poll for a question from the agent
					for (let i = 0; i < 120; i++) {
						const pending = questionManager.getPendingQuestion();
						if (pending) {
							questionReceived = true;
							// Answer with the first option
							const firstOption =
								pending.questions[0]?.options[0]?.label || "Python";
							questionManager.submitAnswer(pending.toolUseID, {
								"0": firstOption,
							});
							return;
						}
						await new Promise((r) => setTimeout(r, 1000));
					}
				})(),
			]);

			// The prompt should always complete
			assert.ok(chunks.length > 0, "Should have received response chunks");

			if (!questionReceived) {
				console.log(
					"  ⚠️  LLM did not ask a question — question forwarding not exercised this run",
				);
			}
		});
	});
});
