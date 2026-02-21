import type { Agent } from "../agent/interface.js";
import { ClaudeSDKClient } from "../claude-sdk/client.js";
import { OpenCodeClient } from "../opencode-sdk/client.js";

export interface AgentOptions {
	cwd: string;
	model?: string;
	env: Record<string, string>;
}

const AGENT_FACTORIES: Record<string, (options: AgentOptions) => Agent> = {
	"claude-code": (opts) =>
		new ClaudeSDKClient({
			cwd: opts.cwd,
			model: opts.model,
			env: opts.env,
		}),
	opencode: (opts) =>
		new OpenCodeClient({
			cwd: opts.cwd,
			model: opts.model,
			env: opts.env,
		}),
};

export function createAgent(agentType: string, options: AgentOptions): Agent {
	const factory = AGENT_FACTORIES[agentType];
	if (!factory) {
		throw new Error(`Unknown agent type: ${agentType}`);
	}
	return factory(options);
}

export function isValidAgentType(agentType: string): boolean {
	return agentType in AGENT_FACTORIES;
}
