import useSWR from "swr";
import { api } from "../api-client";
import type {
	MCPServer,
	CreateMCPServerRequest,
	UpdateMCPServerRequest,
	Skill,
} from "../api-types";

export function useMCPServers() {
	const { data, error, isLoading, mutate } = useSWR("mcp-servers", () =>
		api.getMCPServers(),
	);

	const createMCPServer = async (payload: CreateMCPServerRequest) => {
		const server = await api.createMCPServer(payload);
		mutate();
		return server;
	};

	const updateMCPServer = async (
		id: string,
		payload: UpdateMCPServerRequest,
	) => {
		const server = await api.updateMCPServer(id, payload);
		mutate();
		return server;
	};

	const deleteMCPServer = async (id: string) => {
		await api.deleteMCPServer(id);
		mutate();
	};

	return {
		servers: (data?.servers || []) as MCPServer[],
		isLoading,
		error,
		createMCPServer,
		updateMCPServer,
		deleteMCPServer,
		mutate,
	};
}

export function useAgentSkills(agentId: string | undefined) {
	const { data, error, isLoading, mutate } = useSWR(
		agentId ? ["agent-skills", agentId] : null,
		() => api.getAgentSkills(agentId as string),
	);

	const attachSkill = async (skillId: string) => {
		if (!agentId) return;
		await api.attachSkill(agentId, skillId);
		mutate();
	};

	const detachSkill = async (skillId: string) => {
		if (!agentId) return;
		await api.detachSkill(agentId, skillId);
		mutate();
	};

	return {
		skills: (data?.skills || []) as Skill[],
		isLoading,
		error,
		attachSkill,
		detachSkill,
		mutate,
	};
}

export function useAgentMCPServers(agentId: string | undefined) {
	const { data, error, isLoading, mutate } = useSWR(
		agentId ? ["agent-mcp-servers", agentId] : null,
		() => api.getAgentMCPServers(agentId as string),
	);

	const attachMCPServer = async (mcpServerId: string) => {
		if (!agentId) return;
		await api.attachMCPServer(agentId, mcpServerId);
		mutate();
	};

	const detachMCPServer = async (mcpServerId: string) => {
		if (!agentId) return;
		await api.detachMCPServer(agentId, mcpServerId);
		mutate();
	};

	return {
		servers: (data?.servers || []) as MCPServer[],
		isLoading,
		error,
		attachMCPServer,
		detachMCPServer,
		mutate,
	};
}
