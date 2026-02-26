import assert from "node:assert";
import { existsSync, mkdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, before, beforeEach, describe, it } from "node:test";
import type { DynamicToolUIPart } from "ai";
import { clearAllSessionMappings as clearStoredSession } from "../store/session.js";
import { ClaudeSDKClient } from "./client.js";

describe("ClaudeSDKClient", () => {
	let client: ClaudeSDKClient;

	beforeEach(() => {
		client = new ClaudeSDKClient({
			cwd: "/home/user/workspace",
			model: "claude-sonnet-4-5-20250929",
			env: { TEST_VAR: "test" },
		});
	});

	describe("constructor", () => {
		it("initializes with provided options", () => {
			const client = new ClaudeSDKClient({
				cwd: "/test/path",
				model: "claude-opus-4-5-20251101",
				env: { FOO: "bar" },
			});

			const env = client.getEnvironment();
			assert.strictEqual(env.FOO, "bar");
		});

		it("copies environment to avoid mutations", () => {
			const originalEnv = { FOO: "bar" };
			const client = new ClaudeSDKClient({
				cwd: "/test",
				env: originalEnv,
			});

			const env = client.getEnvironment();
			env.FOO = "mutated";

			// Original should be unchanged
			assert.strictEqual(originalEnv.FOO, "bar");
		});
	});

	describe("ensureSession", () => {
		it("creates a session context for the provided id", async () => {
			const ctx = await client.ensureSession("my-session-id");

			assert.ok(ctx, "Should return a session context");
		});

		it("reuses existing session context for the same id", async () => {
			const ctx1 = await client.ensureSession("test-session");
			const ctx2 = await client.ensureSession("test-session");

			assert.strictEqual(ctx1, ctx2);
		});
	});

	describe("environment management", () => {
		it("updateEnvironment merges new values", async () => {
			await client.updateEnvironment("default", { NEW_VAR: "new_value" });

			const env = client.getEnvironment();
			assert.strictEqual(env.TEST_VAR, "test"); // Original preserved
			assert.strictEqual(env.NEW_VAR, "new_value"); // New added
		});

		it("updateEnvironment overwrites existing values", async () => {
			await client.updateEnvironment("default", { TEST_VAR: "updated" });

			const env = client.getEnvironment();
			assert.strictEqual(env.TEST_VAR, "updated");
		});

		it("getEnvironment returns copy", () => {
			const env1 = client.getEnvironment();
			env1.MUTATED = "value";

			const env2 = client.getEnvironment();
			assert.strictEqual(env2.MUTATED, undefined);
		});
	});

	describe("per-session model", () => {
		it("different session ids create independent sessions", async () => {
			const ctx1 = await client.ensureSession("session-1");
			const ctx2 = await client.ensureSession("session-2");

			assert.notStrictEqual(ctx1, ctx2);
		});
	});

	describe("cancel", () => {
		it("cancel is a no-op when no active prompt", async () => {
			await client.cancel("some-session");
			// Should not throw
		});

		it("cancel with session id is a no-op when no active prompt", async () => {
			await client.ensureSession("test");
			await client.cancel("test");
			// Should not throw
		});
	});

	// Note: Tests for prompt(), discoverAvailableSessions(), and loadFullSession()
	// require actual SDK integration or complex mocking. These should be tested
	// in integration tests where we can use real session files and SDK responses.
	// See test/integration/ for these tests.
});

describe("ClaudeSDKClient session restoration after restart", () => {
	// This test simulates an agent-api restart scenario where:
	// 1. A session existed with messages before restart
	// 2. After restart, the Go server passes the native Claude session ID via header
	// 3. The agent-api calls ensureSession with the native ID to load messages
	//
	// The test creates Claude SDK session files:
	// - ~/.claude/projects/<encoded-cwd>/<sessionId>.jsonl (actual messages)

	const TEST_DATA_DIR = join(tmpdir(), `discobot-restart-test-${process.pid}`);
	const CLAUDE_PROJECTS_DIR = join(TEST_DATA_DIR, ".claude", "projects");
	const CONFIG_DIR = join(TEST_DATA_DIR, ".config", "discobot");
	const testSessionFile = join(CONFIG_DIR, "agent-session.json");
	const testMessagesFile = join(CONFIG_DIR, "agent-messages.json");

	// Test workspace path - we encode this for the Claude projects dir
	const TEST_CWD = "/home/testuser/myproject";
	const ENCODED_CWD = "-home-testuser-myproject";
	const CLAUDE_SESSION_ID = "test-claude-session-uuid-123";
	let savedHome: string | undefined;
	let savedUserProfile: string | undefined;

	before(() => {
		// Create test directories
		mkdirSync(join(CLAUDE_PROJECTS_DIR, ENCODED_CWD), { recursive: true });
		mkdirSync(CONFIG_DIR, { recursive: true });

		// Set env vars to use test files
		process.env.SESSION_FILE = testSessionFile;
		process.env.MESSAGES_FILE = testMessagesFile;
		// Save and override home directory (USERPROFILE takes precedence on Windows)
		savedHome = process.env.HOME;
		savedUserProfile = process.env.USERPROFILE;
		process.env.HOME = TEST_DATA_DIR;
		process.env.USERPROFILE = TEST_DATA_DIR;
	});

	beforeEach(async () => {
		// Clear persisted session before each test
		await clearStoredSession();
		// Remove test files if they exist
		if (existsSync(testSessionFile)) {
			rmSync(testSessionFile);
		}
		if (existsSync(testMessagesFile)) {
			rmSync(testMessagesFile);
		}
		// Remove all Claude session files in the test directory
		const claudeSessionDir = join(CLAUDE_PROJECTS_DIR, ENCODED_CWD);
		if (existsSync(claudeSessionDir)) {
			rmSync(claudeSessionDir, { recursive: true, force: true });
			mkdirSync(claudeSessionDir, { recursive: true });
		}
	});

	after(async () => {
		// Clean up
		await clearStoredSession();
		if (existsSync(TEST_DATA_DIR)) {
			rmSync(TEST_DATA_DIR, { recursive: true, force: true });
		}
		// Restore env vars
		delete process.env.SESSION_FILE;
		delete process.env.MESSAGES_FILE;
		process.env.HOME = savedHome;
		if (savedUserProfile !== undefined) {
			process.env.USERPROFILE = savedUserProfile;
		} else {
			delete process.env.USERPROFILE;
		}
	});

	/**
	 * Create a Claude SDK session file in JSONL format with test messages.
	 * This simulates what Claude SDK writes during a conversation.
	 */
	async function createClaudeSessionFile(
		claudeSessionId: string,
		messages: Array<{ type: string; uuid: string; message: unknown }>,
	): Promise<void> {
		const sessionFile = join(
			CLAUDE_PROJECTS_DIR,
			ENCODED_CWD,
			`${claudeSessionId}.jsonl`,
		);
		const lines = messages.map((m) => JSON.stringify(m));
		const { writeFile: writeFileAsync } = await import("node:fs/promises");
		await writeFileAsync(sessionFile, lines.join("\n"), "utf-8");
	}

	it("restores messages after restart when native session ID is known", async () => {
		// Step 1: Create a Claude session file with messages (simulating previous session)
		await createClaudeSessionFile(CLAUDE_SESSION_ID, [
			{
				type: "user",
				uuid: "user-msg-1",
				message: {
					role: "user",
					content: "Hello, how are you?",
				},
			},
			{
				type: "assistant",
				uuid: "asst-msg-1",
				message: {
					id: "asst-msg-1",
					role: "assistant",
					content: [{ type: "text", text: "I am doing well, thank you!" }],
				},
			},
		]);

		// Step 2: Create a new client (simulating restart)
		// The Go server now tracks the agent session ID and passes it directly.
		const client = new ClaudeSDKClient({
			cwd: TEST_CWD,
		});

		// Step 3: Call ensureSession with the session ID (maps to the Claude session file on disk)
		await client.ensureSession(CLAUDE_SESSION_ID);

		// Step 4: Verify messages are loaded
		const messages = await client.getMessages(CLAUDE_SESSION_ID);
		assert.strictEqual(
			messages.length,
			2,
			"Should have restored 2 messages from disk",
		);

		// Verify user message
		const userMsg = messages.find((m) => m.role === "user");
		assert.ok(userMsg, "User message should be present");
		assert.strictEqual(userMsg.id, "user-msg-1");

		// Verify assistant message
		const asstMsg = messages.find((m) => m.role === "assistant");
		assert.ok(asstMsg, "Assistant message should be present");
		assert.strictEqual(asstMsg.id, "asst-msg-1");
	});

	it("returns empty messages when no session file exists on disk", async () => {
		// No session file exists - simulating a new session

		// Create a new client
		const client = new ClaudeSDKClient({
			cwd: TEST_CWD,
		});

		// Call ensureSession with a session ID that has no file on disk
		await client.ensureSession("brand-new-session");

		// Should have no messages
		const messages = await client.getMessages("brand-new-session");
		assert.strictEqual(
			messages.length,
			0,
			"Should have no messages for a new session",
		);
	});

	it("returns empty messages when Claude session file does not exist", async () => {
		// Create a new client
		const client = new ClaudeSDKClient({
			cwd: TEST_CWD,
		});

		// Call ensureSession with a session ID that has no file on disk
		await client.ensureSession("non-existent-session-id");

		// Should have no messages (file doesn't exist)
		const messages = await client.getMessages("non-existent-session-id");
		assert.strictEqual(
			messages.length,
			0,
			"Should have no messages when Claude session file is missing",
		);
	});

	it("does not auto-adopt when multiple Claude sessions exist (ambiguous)", async () => {
		// Create TWO session files — auto-adopt only triggers for exactly 1 session
		await createClaudeSessionFile(CLAUDE_SESSION_ID, [
			{
				type: "user",
				uuid: "user-msg-1",
				message: { role: "user", content: "Hello" },
			},
		]);
		await createClaudeSessionFile("other-claude-session-uuid", [
			{
				type: "user",
				uuid: "user-msg-2",
				message: { role: "user", content: "World" },
			},
		]);

		// Create a new client
		const client = new ClaudeSDKClient({
			cwd: TEST_CWD,
		});

		// With multiple sessions no auto-adopt occurs — new session starts empty
		await client.ensureSession("different-session-id");

		const messages = await client.getMessages("different-session-id");
		assert.strictEqual(
			messages.length,
			0,
			"Should not auto-adopt when multiple sessions exist",
		);
	});

	it("restores messages with tool calls after restart", async () => {
		// Create a session with tool calls
		await createClaudeSessionFile(CLAUDE_SESSION_ID, [
			{
				type: "user",
				uuid: "user-msg-1",
				message: { role: "user", content: "List files in current directory" },
			},
			{
				type: "assistant",
				uuid: "asst-msg-1",
				message: {
					id: "asst-msg-1",
					role: "assistant",
					content: [
						{ type: "text", text: "Let me list the files for you." },
						{
							type: "tool_use",
							id: "tool-1",
							name: "Bash",
							input: { command: "ls -la" },
						},
					],
				},
			},
			{
				type: "user",
				uuid: "user-msg-2",
				message: {
					role: "user",
					content: [
						{
							type: "tool_result",
							tool_use_id: "tool-1",
							content: "file1.txt\nfile2.txt",
						},
					],
				},
			},
		]);

		// Create a new client — the Go server now provides the native session ID directly
		const client = new ClaudeSDKClient({
			cwd: TEST_CWD,
		});

		// Ensure session with the native Claude session ID
		await client.ensureSession(CLAUDE_SESSION_ID);

		// Get messages
		const messages = await client.getMessages(CLAUDE_SESSION_ID);

		// Should have restored messages
		assert.ok(messages.length >= 2, "Should have restored messages");

		// Find assistant message with tool call
		const asstMsg = messages.find((m) => m.role === "assistant");
		assert.ok(asstMsg, "Should have assistant message");

		// Check for tool part
		const toolPart = asstMsg.parts.find((p) => p.type === "dynamic-tool") as
			| DynamicToolUIPart
			| undefined;
		assert.ok(toolPart, "Should have tool part in assistant message");
		assert.strictEqual(toolPart.toolName, "Bash");
		assert.strictEqual(toolPart.toolCallId, "tool-1");
	});

	it("restores tool outputs merged into tool parts", async () => {
		// Create a session with tool call AND tool result
		// This tests the fix for tool outputs not being merged when loading from disk
		await createClaudeSessionFile(CLAUDE_SESSION_ID, [
			{
				type: "user",
				uuid: "user-msg-1",
				message: { role: "user", content: "What files are here?" },
			},
			{
				type: "assistant",
				uuid: "asst-msg-1",
				message: {
					id: "msg-1",
					role: "assistant",
					content: [
						{
							type: "tool_use",
							id: "tool-abc",
							name: "Bash",
							input: { command: "ls" },
						},
					],
				},
			},
			{
				// Tool result comes as a user message with no text content
				type: "user",
				uuid: "user-msg-2",
				message: {
					role: "user",
					content: [
						{
							type: "tool_result",
							tool_use_id: "tool-abc",
							content: "file1.txt\nfile2.txt\nfile3.txt",
						},
					],
				},
			},
			{
				// Assistant continues after tool result
				type: "assistant",
				uuid: "asst-msg-2",
				message: {
					id: "msg-2",
					role: "assistant",
					content: [{ type: "text", text: "I found 3 files." }],
				},
			},
		]);

		// Create a new client and load session with the native Claude session ID
		const client = new ClaudeSDKClient({ cwd: TEST_CWD });
		await client.ensureSession(CLAUDE_SESSION_ID);

		const messages = await client.getMessages(CLAUDE_SESSION_ID);

		// Find the assistant message (should be merged into one)
		const asstMsg = messages.find((m) => m.role === "assistant");
		assert.ok(asstMsg, "Should have assistant message");

		// Find the tool part
		const toolPart = asstMsg.parts.find((p) => p.type === "dynamic-tool") as
			| DynamicToolUIPart
			| undefined;
		assert.ok(toolPart, "Should have tool part");

		// THE KEY ASSERTION: Tool output should be merged into the part
		assert.strictEqual(
			toolPart.state,
			"output-available",
			"Tool part should have output-available state",
		);
		assert.strictEqual(
			toolPart.output,
			"file1.txt\nfile2.txt\nfile3.txt",
			"Tool output should be merged into the part",
		);
	});

	it("restores tool error state when tool result has is_error", async () => {
		await createClaudeSessionFile(CLAUDE_SESSION_ID, [
			{
				type: "user",
				uuid: "user-msg-1",
				message: { role: "user", content: "Delete all files" },
			},
			{
				type: "assistant",
				uuid: "asst-msg-1",
				message: {
					id: "msg-1",
					role: "assistant",
					content: [
						{
							type: "tool_use",
							id: "tool-err",
							name: "Bash",
							input: { command: "rm -rf /" },
						},
					],
				},
			},
			{
				type: "user",
				uuid: "user-msg-2",
				message: {
					role: "user",
					content: [
						{
							type: "tool_result",
							tool_use_id: "tool-err",
							content: "Permission denied",
							is_error: true,
						},
					],
				},
			},
		]);

		// No mapping needed — the Go server now passes the native session ID directly
		const client = new ClaudeSDKClient({ cwd: TEST_CWD });
		await client.ensureSession(CLAUDE_SESSION_ID);

		const messages = await client.getMessages(CLAUDE_SESSION_ID);
		const asstMsg = messages.find((m) => m.role === "assistant");
		assert.ok(asstMsg, "Should have assistant message");
		const toolPart = asstMsg.parts.find((p) => p.type === "dynamic-tool") as
			| DynamicToolUIPart
			| undefined;
		assert.ok(toolPart, "Should have tool part");

		assert.strictEqual(
			toolPart.state,
			"output-error",
			"Tool part should have output-error state",
		);
		assert.strictEqual(
			toolPart.errorText,
			"Permission denied",
			"Tool error text should be set",
		);
	});

	it("creates new session when no Claude sessions exist", async () => {
		// No Claude session files exist

		const client = new ClaudeSDKClient({
			cwd: TEST_CWD,
		});

		const sessionId = "new-session-id";
		await client.ensureSession(sessionId);

		const messages = await client.getMessages(sessionId);
		assert.strictEqual(
			messages.length,
			0,
			"New session should have no messages",
		);
	});

	it("creates new session when multiple Claude sessions exist but none match", async () => {
		// Create multiple Claude session files
		await createClaudeSessionFile("session-1", [
			{
				type: "user",
				uuid: "user-1",
				message: { role: "user", content: "Session 1" },
			},
		]);
		await createClaudeSessionFile("session-2", [
			{
				type: "user",
				uuid: "user-2",
				message: { role: "user", content: "Session 2" },
			},
		]);

		const client = new ClaudeSDKClient({
			cwd: TEST_CWD,
		});

		// A brand-new session ID should not load any of the existing session files
		const sessionId = "brand-new-session-id";
		await client.ensureSession(sessionId);

		const messages = await client.getMessages(sessionId);
		assert.strictEqual(
			messages.length,
			0,
			"New default session should have no messages",
		);
	});

	describe("prompt error handling", () => {
		// These tests verify the error detection logic when Claude process exits
		// We test this by creating session files with errors and checking detection

		it("detects and returns user-friendly error when session file has API error", async () => {
			const CLAUDE_SESSION_ID = "claude-error-session-123";

			// Create session file with API error (simulating what SDK writes on crash)
			await createClaudeSessionFile(CLAUDE_SESSION_ID, [
				{
					type: "user",
					uuid: "user-msg-error",
					message: {
						role: "user",
						content: "test message",
					},
				},
				{
					type: "assistant",
					uuid: "asst-msg-error",
					error: "authentication_failed",
					isApiErrorMessage: true,
					message: {
						id: "asst-msg-error",
						role: "assistant",
						content: [
							{
								type: "text",
								text: "Invalid API key · Fix external API key",
							},
						],
					},
				} as { type: string; uuid: string; message: unknown },
			]);

			// Test that getLastMessageError detects the error
			const { getLastMessageError } = await import("./persistence.js");
			const error = await getLastMessageError(CLAUDE_SESSION_ID, TEST_CWD);

			assert.strictEqual(
				error,
				"Invalid API key · Fix external API key",
				"Should detect user-friendly error message",
			);
		});

		it("detects error with only error field and content text", async () => {
			const CLAUDE_SESSION_ID = "error-only-field-123";

			await createClaudeSessionFile(CLAUDE_SESSION_ID, [
				{
					type: "assistant",
					uuid: "asst-error-only",
					error: "rate_limit_exceeded",
					message: {
						id: "asst-error-only",
						role: "assistant",
						content: [
							{
								type: "text",
								text: "Rate limit exceeded. Please try again later.",
							},
						],
					},
				} as { type: string; uuid: string; message: unknown },
			]);

			const { getLastMessageError } = await import("./persistence.js");
			const error = await getLastMessageError(CLAUDE_SESSION_ID, TEST_CWD);

			assert.strictEqual(
				error,
				"Rate limit exceeded. Please try again later.",
				"Should extract user-friendly text when error field present",
			);
		});

		it("does not detect error patterns in content without explicit error field", async () => {
			const CLAUDE_SESSION_ID = "error-pattern-123";

			await createClaudeSessionFile(CLAUDE_SESSION_ID, [
				{
					type: "assistant",
					uuid: "asst-pattern",
					message: {
						id: "asst-pattern",
						role: "assistant",
						content: [
							{
								type: "text",
								text: "Error: Connection timeout occurred while processing request",
							},
						],
					},
				} as { type: string; uuid: string; message: unknown },
			]);

			const { getLastMessageError } = await import("./persistence.js");
			const error = await getLastMessageError(CLAUDE_SESSION_ID, TEST_CWD);

			assert.strictEqual(
				error,
				null,
				"Should not detect error pattern in text content without explicit error field",
			);
		});

		it("returns null when no error is present in session", async () => {
			const CLAUDE_SESSION_ID = "no-error-123";

			await createClaudeSessionFile(CLAUDE_SESSION_ID, [
				{
					type: "user",
					uuid: "user-normal",
					message: {
						role: "user",
						content: "hello",
					},
				},
				{
					type: "assistant",
					uuid: "asst-normal",
					message: {
						id: "asst-normal",
						role: "assistant",
						content: [
							{
								type: "text",
								text: "Hello! How can I help you today?",
							},
						],
					},
				} as { type: string; uuid: string; message: unknown },
			]);

			const { getLastMessageError } = await import("./persistence.js");
			const error = await getLastMessageError(CLAUDE_SESSION_ID, TEST_CWD);

			assert.strictEqual(error, null, "Should return null for normal messages");
		});

		it("falls back to error code when no content text available", async () => {
			const CLAUDE_SESSION_ID = "error-no-content-123";

			await createClaudeSessionFile(CLAUDE_SESSION_ID, [
				{
					type: "assistant",
					uuid: "asst-no-content",
					error: "internal_server_error",
					isApiErrorMessage: true,
					message: {
						id: "asst-no-content",
						role: "assistant",
						content: [], // Empty content array
					},
				} as { type: string; uuid: string; message: unknown },
			]);

			const { getLastMessageError } = await import("./persistence.js");
			const error = await getLastMessageError(CLAUDE_SESSION_ID, TEST_CWD);

			assert.strictEqual(
				error,
				"internal_server_error",
				"Should fall back to error code when no content text",
			);
		});
	});
});
