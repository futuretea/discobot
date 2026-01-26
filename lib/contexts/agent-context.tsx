"use client";

import * as React from "react";
import type { Agent, SupportedAgentType } from "@/lib/api-types";
import { useAgentTypes } from "@/lib/hooks/use-agent-types";
import { useAgents } from "@/lib/hooks/use-agents";

export interface AgentContextValue {
	// Data
	agents: Agent[];
	agentTypes: SupportedAgentType[];
	isLoading: boolean;

	// Selection
	selectedAgentId: string | null;
	selectAgent: (agentId: string | null) => void;

	// Mutations
	createAgent: ReturnType<typeof useAgents>["createAgent"];
	updateAgent: ReturnType<typeof useAgents>["updateAgent"];
	mutate: ReturnType<typeof useAgents>["mutate"];
}

const AgentContext = React.createContext<AgentContextValue | null>(null);

export function useAgentContext() {
	const context = React.useContext(AgentContext);
	if (!context) {
		throw new Error("useAgentContext must be used within an AgentProvider");
	}
	return context;
}

interface AgentProviderProps {
	children: React.ReactNode;
}

export function AgentProvider({ children }: AgentProviderProps) {
	const { agents, isLoading, createAgent, updateAgent, mutate } = useAgents();
	const { agentTypes } = useAgentTypes();

	const [selectedAgentId, setSelectedAgentId] = React.useState<string | null>(
		null,
	);

	const selectAgent = React.useCallback((agentId: string | null) => {
		setSelectedAgentId(agentId);
	}, []);

	const value = React.useMemo<AgentContextValue>(
		() => ({
			agents,
			agentTypes,
			isLoading,
			selectedAgentId,
			selectAgent,
			createAgent,
			updateAgent,
			mutate,
		}),
		[
			agents,
			agentTypes,
			isLoading,
			selectedAgentId,
			selectAgent,
			createAgent,
			updateAgent,
			mutate,
		],
	);

	return (
		<AgentContext.Provider value={value}>{children}</AgentContext.Provider>
	);
}
