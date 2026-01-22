"use client";

import useSWR from "swr";
import { api } from "../api-client";
import type { UpdateSessionRequest } from "../api-types";

export function useSessions(
	workspaceId: string | null,
	options?: { includeClosed?: boolean },
) {
	const includeClosed = options?.includeClosed ?? false;
	const { data, error, isLoading, mutate } = useSWR(
		workspaceId ? `sessions-${workspaceId}-${includeClosed}` : null,
		() =>
			workspaceId ? api.getSessions(workspaceId, { includeClosed }) : null,
	);

	return {
		sessions: data?.sessions || [],
		isLoading,
		error,
		mutate,
	};
}

export function useSession(sessionId: string | null) {
	const { data, error, isLoading, mutate } = useSWR(
		sessionId ? `session-${sessionId}` : null,
		() => (sessionId ? api.getSession(sessionId) : null),
	);

	const updateSession = async (data: UpdateSessionRequest) => {
		if (!sessionId) return;
		const session = await api.updateSession(sessionId, data);
		mutate();
		return session;
	};

	return {
		session: data,
		isLoading,
		error,
		updateSession,
		mutate,
	};
}

// NOTE: useCreateSession removed - sessions are created implicitly via /chat endpoint

export function useDeleteSession() {
	/**
	 * Delete a session. The session will transition to "removing" state
	 * and be removed from the cache when the SSE event with status=removed arrives.
	 * @param sessionId - The session ID to delete
	 */
	const deleteSession = async (sessionId: string) => {
		await api.deleteSession(sessionId);
		// Don't invalidate caches here - the session will show "removing" state
		// and be removed from cache when we receive the session_updated event
		// with status=removed via SSE
	};

	return { deleteSession };
}
