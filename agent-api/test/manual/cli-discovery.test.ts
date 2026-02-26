/**
 * Test Claude CLI discovery from PATH
 * Usage: tsx scripts/test-cli-discovery.ts
 *
 * ClaudeSDKClient uses lazy setup — the CLI is discovered on first prompt().
 * We test by calling ensureSession which doesn't require the CLI.
 */

import { ClaudeSDKClient } from "../../src/claude-sdk/client.js";

console.log("Testing Claude CLI discovery...\n");

const client = new ClaudeSDKClient({
	cwd: process.cwd(),
	model: "claude-sonnet-4-5-20250929",
	env: process.env as Record<string, string>,
});

try {
	console.log("Creating session context (lazy setup)...");
	const ctx = await client.ensureSession("test-session");
	console.log("✓ Session context created");
	console.log(`✓ NativeId: ${ctx.nativeId ?? "(none yet)"}`);

	console.log("\n✓ Test completed successfully");
} catch (error) {
	console.error("\n❌ Error:", error);
	process.exit(1);
}
