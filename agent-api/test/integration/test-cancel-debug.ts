/**
 * Debug test for cancellation behavior
 */

import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { ClaudeSDKClient } from "../../src/claude-sdk/client.js";

describe("Cancellation Debug", () => {
	it("can continue after cancel", async () => {
		const client = new ClaudeSDKClient({
			cwd: process.cwd(),
			env: process.env as Record<string, string>,
		});

		const sessionId = "cancel-debug";
		await client.ensureSession(sessionId);

		console.log("\n1. Starting first prompt...");
		const gen1 = client.prompt(
			{
				id: "msg-1",
				role: "user",
				parts: [{ type: "text", text: "Count from 1 to 100 slowly" }],
			},
			sessionId,
		);

		// Get a few chunks then cancel
		const iter = gen1[Symbol.asyncIterator]();
		console.log("2. Getting first chunk...");
		await iter.next();
		console.log("3. Getting second chunk...");
		await iter.next();

		console.log("4. Calling cancel()...");
		await client.cancel(sessionId);
		console.log("5. Cancel returned (agent restarted)");

		// Note: Don't try to iterate the old generator after cancel/restart
		// The old generator is connected to the dead CLI process

		console.log("6. Trying to send a new prompt...");
		const gen2 = client.prompt(
			{
				id: "msg-2",
				role: "user",
				parts: [{ type: "text", text: "Say 'Hello after cancel'" }],
			},
			sessionId,
		);

		console.log("8. Collecting response from second prompt...");
		const chunks = [];
		try {
			for await (const chunk of gen2) {
				chunks.push(chunk);
				console.log(`   Chunk ${chunks.length}: ${chunk.type}`);
				if (chunks.length > 20) break; // Safety limit
			}
			console.log(`9. SUCCESS! Got ${chunks.length} chunks from second prompt`);
			assert.ok(chunks.length > 0, "Should get response after cancel");
		} catch (error) {
			console.log(`9. ERROR collecting chunks: ${error}`);
			console.log(`   Got ${chunks.length} chunks before error`);
			throw error;
		}
	});
});
