/**
 * Single simple test to debug timeout issues
 */

import assert from "node:assert/strict";
import { afterEach, beforeEach, describe, it } from "node:test";
import { ClaudeSDKClient } from "../../src/claude-sdk/client.js";

describe("Single Test", () => {
	let agent: ClaudeSDKClient;

	beforeEach(async () => {
		agent = new ClaudeSDKClient({
			cwd: process.cwd(),
			env: process.env as Record<string, string>,
		});
	});

	afterEach(async () => {
		// ClaudeSDKClient has no explicit disconnect — uses lazy setup
	});

	it("creates session context without explicit connect", async () => {
		console.log("[TEST] Creating session context");
		const ctx = await agent.ensureSession("test-session");
		assert.ok(ctx, "Should return a session context");
		console.log("[TEST] Session context created successfully");
	});
});
