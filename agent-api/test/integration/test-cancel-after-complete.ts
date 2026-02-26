/**
 * Test cancelling a SECOND prompt after first prompt completes
 * This should preserve the history from the first completed turn
 */

import type { UIMessageChunk } from "../../src/api/types.js";
import { ClaudeSDKClient } from "../../src/claude-sdk/client.js";

async function collectChunks(gen: AsyncGenerator<UIMessageChunk>) {
	const chunks = [];
	for await (const chunk of gen) {
		chunks.push(chunk);
	}
	return chunks;
}

async function main() {
	console.log("=== Testing cancel after complete turn ===\n");

	const agent = new ClaudeSDKClient({
		cwd: process.cwd(),
		env: process.env as Record<string, string>,
	});

	console.log("1. Creating session...");
	const sessionId = "test-session-id";
	console.log(`   Session: ${sessionId}\n`);

	const message = {
		id: "msg-1",
		role: "user" as const,
		parts: [{ type: "text" as const, text: "Say exactly: 'FIRST'" }],
	};

	console.log("3. Completing first prompt...");
	const gen1 = agent.prompt(message, sessionId);
	const chunks1 = await collectChunks(gen1);
	console.log(`   ✓ First prompt completed with ${chunks1.length} chunks\n`);

	console.log("4. Starting second prompt...");
	const message2 = {
		id: "msg-2",
		role: "user" as const,
		parts: [{ type: "text" as const, text: "Say exactly: 'SECOND'" }],
	};

	const gen2 = agent.prompt(message2, sessionId);
	const iter = gen2[Symbol.asyncIterator]();

	console.log("5. Getting first chunk of second prompt...");
	await iter.next();
	console.log("   ✓ Got first chunk\n");

	console.log("6. Cancelling second prompt...");
	await agent.cancel(sessionId);
	console.log("   ✓ Cancelled\n");

	console.log("7. Checking session has first turn history...");
	const sessionMessages = await agent.getMessages(sessionId);
	console.log(`   Messages in session: ${sessionMessages.length}`);

	if (sessionMessages.length >= 2) {
		console.log("   ✓ History preserved!\n");
	} else {
		console.log("   ✗ History lost!\n");
	}

	console.log("8. Starting third prompt...");
	const message3 = {
		id: "msg-3",
		role: "user" as const,
		parts: [
			{
				type: "text" as const,
				text: "What was the first thing I asked you to say?",
			},
		],
	};

	try {
		const gen3 = agent.prompt(message3, sessionId);
		const chunks3 = await collectChunks(gen3);
		console.log(`   ✓ Third prompt succeeded with ${chunks3.length} chunks\n`);

		// Check if it remembers "FIRST"
		const text = chunks3
			.filter((c) => c.type === "text-delta" && "delta" in c)
			.map((c) => ("delta" in c ? c.delta : ""))
			.join("");

		if (text.includes("FIRST")) {
			console.log("   ✓ Session remembered first turn!\n");
		} else {
			console.log("   ⚠️  Session may not have history context\n");
		}

		console.log("=== Test completed successfully ===");
	} catch (error) {
		console.error(
			`   ✗ Third prompt failed: ${error instanceof Error ? error.message : error}\n`,
		);
		throw error;
	}
}

main().catch((error) => {
	console.error("\n=== TEST FAILED ===");
	console.error(error);
	process.exit(1);
});
