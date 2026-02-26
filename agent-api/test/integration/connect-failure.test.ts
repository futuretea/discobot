/**
 * ClaudeSDKClient uses lazy setup via ensureSetup() — there is no explicit
 * connect() / disconnect() lifecycle or isConnected property.
 * Connection errors surface when prompt() is called with an invalid CLI path.
 *
 * These tests verify the lazy-setup behaviour.
 */

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ClaudeSDKClient } from "../../src/claude-sdk/client.js";

describe("Agent Lazy Setup", { timeout: 10000 }, () => {
	it("can be constructed without connecting", () => {
		const client = new ClaudeSDKClient({
			cwd: process.cwd(),
			model: "claude-sonnet-4-5-20250929",
			env: process.env as Record<string, string>,
		});

		assert.ok(client, "Client should be created successfully");
	});

	it("ensureSession returns a SessionContext without explicit connect", async () => {
		const client = new ClaudeSDKClient({
			cwd: process.cwd(),
			model: "claude-sonnet-4-5-20250929",
			env: process.env as Record<string, string>,
		});

		const ctx = await client.ensureSession("test-session");
		assert.ok(ctx, "Should return a session context");
	});

	it("cancel is a no-op when no active prompt", async () => {
		const client = new ClaudeSDKClient({
			cwd: process.cwd(),
			model: "claude-sonnet-4-5-20250929",
			env: process.env as Record<string, string>,
		});

		await client.ensureSession("test-session");
		await client.cancel("test-session");
		// Should not throw
	});
});
