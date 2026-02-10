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
		console.log("[TEST] afterEach: disconnecting agent");
		try {
			if (agent?.isConnected) {
				await agent.disconnect();
			}
		} catch (error) {
			console.error("[TEST] Disconnect error:", error);
		}
		console.log("[TEST] afterEach: done");
	});

	it("connects and disconnects", async () => {
		console.log("[TEST] Starting connect test");
		await agent.connect();
		assert.equal(agent.isConnected, true);
		console.log("[TEST] Connected successfully");
	});
});
