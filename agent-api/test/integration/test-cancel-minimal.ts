/**
 * Minimal reproduction of cancellation issue
 */

import { ClaudeSDKClient } from "../../src/claude-sdk/client.js";

async function main() {
	console.log("=== Starting minimal cancellation test ===\n");

	const agent = new ClaudeSDKClient({
		cwd: process.cwd(),
		env: process.env as Record<string, string>,
	});

	console.log("1. Connecting agent...");
	await agent.connect();
	console.log("   ✓ Connected\n");

	console.log("2. Creating session...");
	const session = agent.createSession();
	const sessionId = session.id;
	console.log(`   ✓ Created session: ${sessionId}\n`);

	console.log("3. Starting first prompt...");
	const message = {
		id: "msg-1",
		role: "user" as const,
		parts: [{ type: "text" as const, text: "Say exactly: 'TEST_OK'" }],
	};

	const gen1 = agent.prompt(message, sessionId);
	const iter = gen1[Symbol.asyncIterator]();

	console.log("4. Getting first chunk...");
	const firstChunk = await iter.next();
	console.log(
		`   ✓ Got first chunk: ${JSON.stringify(firstChunk.value).substring(0, 100)}...\n`,
	);

	console.log("5. Cancelling...");
	await agent.cancel(sessionId);
	console.log("   ✓ Cancelled\n");

	console.log("6. Attempting second prompt after cancel...");
	try {
		const gen2 = agent.prompt(message, sessionId);
		const chunks = [];
		for await (const chunk of gen2) {
			chunks.push(chunk);
		}
		console.log(`   ✓ Second prompt succeeded! Got ${chunks.length} chunks\n`);
	} catch (error) {
		console.error(`   ✗ Second prompt failed!`);
		console.error(
			`   Error: ${error instanceof Error ? error.message : error}\n`,
		);
		throw error;
	}

	console.log("7. Disconnecting...");
	await agent.disconnect();
	console.log("   ✓ Disconnected\n");

	console.log("=== Test completed successfully ===");
}

main().catch((error) => {
	console.error("\n=== TEST FAILED ===");
	console.error(error);
	process.exit(1);
});
