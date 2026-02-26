/**
 * OpenCode Translation Integration Tests
 *
 * Validates that the OpenCode translation layer correctly handles
 * real event streams from an OpenCode server using Anthropic models.
 *
 * Uses claude-haiku-4-5 for fast text/tool tests and
 * claude-sonnet-4-5 for reasoning tests.
 *
 * Usage:
 *   ANTHROPIC_API_KEY=xxx node --import tsx --test test/integration/opencode-translation.test.ts
 */

import assert from "node:assert/strict";
import { after, afterEach, before, beforeEach, describe, it } from "node:test";
import type { UIMessageChunk } from "../../src/api/types.js";
import { OpenCodeClient } from "../../src/opencode-sdk/client.js";

const HAIKU = "anthropic/claude-haiku-4-5-20251001";
const THINKING = "anthropic/claude-sonnet-4-5-20250929";
const TIMEOUT = 120_000;

// Helper: collect all chunks from prompt generator with timeout
async function collectChunks(
	generator: AsyncGenerator<UIMessageChunk>,
	timeout = TIMEOUT,
): Promise<UIMessageChunk[]> {
	const chunks: UIMessageChunk[] = [];
	const timeoutPromise = new Promise<never>((_, reject) =>
		setTimeout(() => reject(new Error(`Timeout after ${timeout}ms`)), timeout),
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

// Helper: extract chunk types as string array
function chunkTypes(chunks: UIMessageChunk[]): string[] {
	return chunks.map((c) => c.type);
}

// Helper: concatenate all text deltas
function collectText(chunks: UIMessageChunk[]): string {
	return chunks
		.filter((c) => c.type === "text-delta")
		.map((c) => (c as { delta: string }).delta)
		.join("");
}

// Helper: concatenate all reasoning deltas
function collectReasoning(chunks: UIMessageChunk[]): string {
	return chunks
		.filter((c) => c.type === "reasoning-delta")
		.map((c) => (c as { delta: string }).delta)
		.join("");
}

let msgCounter = 0;
function makeMessage(text: string) {
	msgCounter++;
	return {
		id: `integ-msg-${msgCounter}`,
		role: "user" as const,
		parts: [{ type: "text" as const, text }],
	};
}

const shouldSkip = !process.env.ANTHROPIC_API_KEY;
if (shouldSkip) {
	console.log(
		"⚠️  Skipping OpenCode translation tests: ANTHROPIC_API_KEY not set",
	);
}

describe("OpenCode Translation Integration", { skip: shouldSkip }, () => {
	let agent: OpenCodeClient;
	let sessionId: string;

	before(async () => {
		agent = new OpenCodeClient({
			cwd: process.cwd(),
			model: HAIKU,
			env: process.env as Record<string, string>,
		});
		await agent.connect();
	});

	after(async () => {
		await agent?.disconnect();
	});

	beforeEach(async () => {
		sessionId = "test-session-id";
	});

	afterEach(async () => {
		try {
			await agent.clearSession(sessionId);
		} catch {
			// Ignore cleanup errors
		}
	});

	// ─── Text Response ──────────────────────────────────────────────

	it("streams a simple text response with correct chunk ordering", async () => {
		const gen = agent.prompt(makeMessage("Say exactly: 'HELLO'"), sessionId);
		const chunks = await collectChunks(gen);
		const types = chunkTypes(chunks);

		console.log("text response types:", types);

		// Must start with 'start' and end with 'finish'
		assert.equal(types[0], "start", "First chunk must be 'start'");
		assert.equal(
			types[types.length - 1],
			"finish",
			"Last chunk must be 'finish'",
		);

		// Must have text lifecycle
		assert.ok(types.includes("text-start"), "Should have text-start");
		assert.ok(types.includes("text-delta"), "Should have text-delta");
		assert.ok(types.includes("text-end"), "Should have text-end");

		// text-start must come before first text-delta
		const textStartIdx = types.indexOf("text-start");
		const firstTextDeltaIdx = types.indexOf("text-delta");
		assert.ok(
			textStartIdx < firstTextDeltaIdx,
			"text-start must precede first text-delta",
		);

		// text-end must come after last text-delta
		const textEndIdx = types.lastIndexOf("text-end");
		const lastTextDeltaIdx = types.lastIndexOf("text-delta");
		assert.ok(
			textEndIdx > lastTextDeltaIdx,
			"text-end must follow last text-delta",
		);

		// Text content should contain HELLO
		const text = collectText(chunks);
		assert.ok(
			text.includes("HELLO"),
			`Text should contain HELLO, got: ${text}`,
		);
	});

	// ─── Reasoning Response ─────────────────────────────────────────

	it("streams reasoning before text content", async () => {
		// Use the thinking model for this test
		const gen = agent.prompt(
			makeMessage(
				"Think step by step about why 2+2=4. Keep your reasoning and answer very brief (1-2 sentences each).",
			),
			sessionId,
			THINKING,
		);
		const chunks = await collectChunks(gen);
		const types = chunkTypes(chunks);

		console.log("reasoning response types:", types);

		// Must have reasoning lifecycle
		assert.ok(types.includes("reasoning-start"), "Should have reasoning-start");
		assert.ok(types.includes("reasoning-delta"), "Should have reasoning-delta");
		assert.ok(types.includes("reasoning-end"), "Should have reasoning-end");

		// Reasoning must come before text
		const reasoningStartIdx = types.indexOf("reasoning-start");
		const textStartIdx = types.indexOf("text-start");
		if (textStartIdx >= 0) {
			assert.ok(
				reasoningStartIdx < textStartIdx,
				"reasoning-start must precede text-start",
			);
		}

		// Reasoning content should be non-empty
		const reasoning = collectReasoning(chunks);
		assert.ok(reasoning.length > 0, "Reasoning content should be non-empty");

		// Reasoning deltas should be reasoning-delta, not text-delta
		// (this validates the reasoningPartIds fix)
		const reasoningEnd = types.indexOf("reasoning-end");
		const chunksBetween = chunks.slice(reasoningStartIdx + 1, reasoningEnd);
		for (const c of chunksBetween) {
			assert.equal(
				c.type,
				"reasoning-delta",
				`Chunks between reasoning-start and reasoning-end should be reasoning-delta, got ${c.type}`,
			);
		}
	});

	// ─── Tool Call Response ──────────────────────────────────────────

	it("streams tool call lifecycle", async () => {
		const gen = agent.prompt(
			makeMessage(
				"Use the Bash tool to run 'echo hello_world' and show the output. Be concise.",
			),
			sessionId,
		);
		const chunks = await collectChunks(gen);
		const types = chunkTypes(chunks);

		console.log("tool call types:", types);

		// Must have tool lifecycle
		assert.ok(
			types.includes("tool-input-start"),
			"Should have tool-input-start",
		);
		assert.ok(
			types.includes("tool-input-available"),
			"Should have tool-input-available",
		);
		assert.ok(
			types.includes("tool-output-available"),
			"Should have tool-output-available",
		);

		// tool-input-start must come before tool-input-available
		const toolStartIdx = types.indexOf("tool-input-start");
		const toolInputIdx = types.indexOf("tool-input-available");
		assert.ok(
			toolStartIdx < toolInputIdx,
			"tool-input-start must precede tool-input-available",
		);

		// tool-input-available must come before tool-output-available
		const toolOutputIdx = types.indexOf("tool-output-available");
		assert.ok(
			toolInputIdx < toolOutputIdx,
			"tool-input-available must precede tool-output-available",
		);

		// Validate tool chunk content
		const toolStart = chunks.find((c) => c.type === "tool-input-start") as {
			toolCallId: string;
			toolName: string;
		};
		assert.ok(toolStart.toolCallId, "tool-input-start should have toolCallId");
		assert.ok(toolStart.toolName, "tool-input-start should have toolName");

		const toolInput = chunks.find((c) => c.type === "tool-input-available") as {
			input: unknown;
		};
		assert.ok(toolInput.input, "tool-input-available should have input");

		const toolOutput = chunks.find(
			(c) => c.type === "tool-output-available",
		) as {
			output: string;
		};
		assert.ok(
			toolOutput.output?.includes("hello_world"),
			`Tool output should contain hello_world, got: ${toolOutput.output}`,
		);
	});

	// ─── Step Lifecycle ─────────────────────────────────────────────

	it("includes step lifecycle events in agentic responses", async () => {
		const gen = agent.prompt(
			makeMessage(
				"Use Bash to run 'echo step_test' and tell me the result. Be concise.",
			),
			sessionId,
		);
		const chunks = await collectChunks(gen);
		const types = chunkTypes(chunks);

		console.log("step lifecycle types:", types);

		// Agentic responses should include step events
		assert.ok(types.includes("start-step"), "Should have start-step");
		assert.ok(types.includes("finish-step"), "Should have finish-step");

		// start-step must come before finish-step
		const stepStartIdx = types.indexOf("start-step");
		const stepFinishIdx = types.indexOf("finish-step");
		assert.ok(
			stepStartIdx < stepFinishIdx,
			"start-step must precede finish-step",
		);
	});

	// ─── Chunk Ordering Invariants ──────────────────────────────────

	it("enforces chunk ordering invariants", async () => {
		const gen = agent.prompt(
			makeMessage("What is 1+1? Answer briefly."),
			sessionId,
		);
		const chunks = await collectChunks(gen);
		const types = chunkTypes(chunks);

		// start is first
		assert.equal(types[0], "start", "First chunk must be 'start'");

		// finish is last
		assert.equal(
			types[types.length - 1],
			"finish",
			"Last chunk must be 'finish'",
		);

		// No text-delta without a preceding text-start
		const startedTextIds = new Set<string>();
		for (const chunk of chunks) {
			if (chunk.type === "text-start" && "id" in chunk) {
				startedTextIds.add(chunk.id as string);
			}
			if (chunk.type === "text-delta" && "id" in chunk) {
				assert.ok(
					startedTextIds.has(chunk.id as string),
					`text-delta for ${chunk.id} without preceding text-start`,
				);
			}
		}

		// No reasoning-delta without a preceding reasoning-start
		const startedReasoningIds = new Set<string>();
		for (const chunk of chunks) {
			if (chunk.type === "reasoning-start" && "id" in chunk) {
				startedReasoningIds.add(chunk.id as string);
			}
			if (chunk.type === "reasoning-delta" && "id" in chunk) {
				assert.ok(
					startedReasoningIds.has(chunk.id as string),
					`reasoning-delta for ${chunk.id} without preceding reasoning-start`,
				);
			}
		}
	});

	// ─── Session Messages After Completion ──────────────────────────

	it("persists messages with correct part types after text response", async () => {
		const gen = agent.prompt(
			makeMessage("Say exactly: 'PERSIST_TEST'"),
			sessionId,
		);
		await collectChunks(gen);

		const messages = await agent.getMessages(sessionId);
		assert.ok(messages.length >= 2, "Should have at least user + assistant");

		// Find assistant message
		const assistant = messages.find((m) => m.role === "assistant");
		assert.ok(assistant, "Should have assistant message");

		// Assistant should have text part
		const textPart = assistant.parts.find((p) => p.type === "text");
		assert.ok(textPart, "Assistant should have text part");
		assert.ok(
			"text" in textPart && (textPart.text as string).includes("PERSIST_TEST"),
			"Text part should contain PERSIST_TEST",
		);
	});

	it("persists tool invocations in session messages", async () => {
		const gen = agent.prompt(
			makeMessage("Use Bash to run 'echo persist_tool'. Be concise."),
			sessionId,
		);
		await collectChunks(gen);

		const messages = await agent.getMessages(sessionId);
		const assistant = messages.find((m) => m.role === "assistant");
		assert.ok(assistant);

		// Should have a tool part (dynamic-tool)
		const toolPart = assistant.parts.find((p) => p.type === "dynamic-tool");
		assert.ok(toolPart, "Assistant should have dynamic-tool part");

		const tool = toolPart as {
			toolCallId: string;
			toolName: string;
			state: string;
		};
		assert.ok(tool.toolCallId, "Tool part should have toolCallId");
		assert.ok(tool.toolName, "Tool part should have toolName");
		assert.equal(
			tool.state,
			"output-available",
			"Tool should be in output-available state",
		);
	});
});
