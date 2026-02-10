/**
 * Provider registry for agent contract testing
 *
 * Add your provider here to run contract tests against it.
 */

import type { Agent } from "../../src/agent/interface.js";
import type { UIMessage } from "../../src/api/types.js";
import { ClaudeSDKClient } from "../../src/claude-sdk/client.js";

export interface ProviderConfig {
	name: string;
	createAgent: () => Agent;
	requiredEnvVars: string[];
	testMessages: {
		simple: UIMessage;
		withTools: UIMessage;
		continuation: UIMessage;
	};
}

/**
 * Registry of all available providers
 * Add your provider implementation here
 */
export const PROVIDERS: Record<string, ProviderConfig> = {
	"claude-sdk": {
		name: "ClaudeSDKClient",
		createAgent: () =>
			new ClaudeSDKClient({
				cwd: process.cwd(),
				model: process.env.AGENT_MODEL,
				env: process.env as Record<string, string>,
			}),
		requiredEnvVars: ["ANTHROPIC_API_KEY"],
		testMessages: {
			simple: {
				id: "msg-1",
				role: "user",
				parts: [{ type: "text", text: "Say exactly: 'TEST_OK'" }],
			},
			withTools: {
				id: "msg-2",
				role: "user",
				parts: [
					{
						type: "text",
						text: "Use Bash to run 'echo hello' and show the output. Be concise.",
					},
				],
			},
			continuation: {
				id: "msg-3",
				role: "user",
				parts: [
					{
						type: "text",
						text: "What did I ask you to say in the first message?",
					},
				],
			},
		},
	},

	// Add more providers here:
	// "my-provider": {
	//   name: "MyProvider",
	//   createAgent: () => new MyProviderAgent({ /* config */ }),
	//   requiredEnvVars: ["MY_API_KEY"],
	//   testMessages: { ... },
	// },
};

export function getProvider(name: string): ProviderConfig {
	const provider = PROVIDERS[name];
	if (!provider) {
		const available = Object.keys(PROVIDERS).join(", ");
		throw new Error(`Unknown provider: ${name}. Available: ${available}`);
	}

	// Check required env vars
	const missing = provider.requiredEnvVars.filter((v) => !process.env[v]);
	if (missing.length > 0) {
		throw new Error(
			`Missing required env vars for ${name}: ${missing.join(", ")}`,
		);
	}

	return provider;
}
