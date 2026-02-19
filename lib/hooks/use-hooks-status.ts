import useSWR from "swr";
import { api } from "../api-client";
import type { HooksStatusResponse } from "../api-types";

export function useHooksStatus(sessionId: string | null) {
	const { data, error, isLoading } = useSWR<HooksStatusResponse | null>(
		sessionId ? `hooks-status-${sessionId}` : null,
		() => (sessionId ? api.getHooksStatus(sessionId) : null),
		{
			refreshInterval: (latestData) => {
				// Fast poll (2s) when hooks are running or pending
				const hooks = Object.values(latestData?.hooks ?? {});
				const hasRunning = hooks.some((h) => h.lastResult === "running");
				const hasPending = (latestData?.pendingHooks?.length ?? 0) > 0;
				return hasRunning || hasPending ? 2000 : 10000;
			},
		},
	);

	return {
		hooksStatus: data ?? null,
		isLoading,
		error,
	};
}
