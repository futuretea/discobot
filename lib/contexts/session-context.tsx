"use client";

import * as React from "react";
import useSWR from "swr";
import { api } from "@/lib/api-client";
import type { Session } from "@/lib/api-types";

export interface SessionContextValue {
	// Data
	selectedSessionId: string | null;
	selectedSession: Session | null | undefined;

	// UI state for session creation flow
	preselectedWorkspaceId: string | null;
	workspaceSelectTrigger: number;
	chatResetTrigger: number;

	// Actions
	selectSession: (sessionId: string | null) => void;
	handleSessionSelect: (session: { id: string }) => void;
	handleNewSession: () => void;
	handleAddSession: (workspaceId: string) => void;
	handleSessionCreated: (sessionId: string) => void;
	mutateSelectedSession: () => void;
}

const SessionContext = React.createContext<SessionContextValue | null>(null);

export function useSessionContext() {
	const context = React.useContext(SessionContext);
	if (!context) {
		throw new Error("useSessionContext must be used within a SessionProvider");
	}
	return context;
}

interface SessionProviderProps {
	children: React.ReactNode;
}

export function SessionProvider({ children }: SessionProviderProps) {
	// Selection state
	const [selectedSessionId, setSelectedSessionId] = React.useState<
		string | null
	>(null);
	const [preselectedWorkspaceId, setPreselectedWorkspaceId] = React.useState<
		string | null
	>(null);
	const [workspaceSelectTrigger, setWorkspaceSelectTrigger] = React.useState(0);
	const [chatResetTrigger, setChatResetTrigger] = React.useState(0);

	// Fetch the selected session via SWR
	const { data: selectedSession, mutate: mutateSelectedSession } = useSWR(
		selectedSessionId ? `session-${selectedSessionId}` : null,
		() => (selectedSessionId ? api.getSession(selectedSessionId) : null),
	);

	// Actions
	const selectSession = React.useCallback((sessionId: string | null) => {
		setSelectedSessionId(sessionId);
	}, []);

	const handleSessionSelect = React.useCallback((session: { id: string }) => {
		setSelectedSessionId(session.id);
		setPreselectedWorkspaceId(null);
	}, []);

	const handleNewSession = React.useCallback(() => {
		setSelectedSessionId(null);
		setPreselectedWorkspaceId(null);
		setChatResetTrigger((prev) => prev + 1);
	}, []);

	const handleAddSession = React.useCallback((workspaceId: string) => {
		setSelectedSessionId(null);
		setPreselectedWorkspaceId(workspaceId);
		setWorkspaceSelectTrigger((prev) => prev + 1);
	}, []);

	const handleSessionCreated = React.useCallback((sessionId: string) => {
		setSelectedSessionId(sessionId);
		setPreselectedWorkspaceId(null);
	}, []);

	const value = React.useMemo<SessionContextValue>(
		() => ({
			selectedSessionId,
			selectedSession: selectedSession ?? null,
			preselectedWorkspaceId,
			workspaceSelectTrigger,
			chatResetTrigger,
			selectSession,
			handleSessionSelect,
			handleNewSession,
			handleAddSession,
			handleSessionCreated,
			mutateSelectedSession: () => mutateSelectedSession(),
		}),
		[
			selectedSessionId,
			selectedSession,
			preselectedWorkspaceId,
			workspaceSelectTrigger,
			chatResetTrigger,
			selectSession,
			handleSessionSelect,
			handleNewSession,
			handleAddSession,
			handleSessionCreated,
			mutateSelectedSession,
		],
	);

	return (
		<SessionContext.Provider value={value}>{children}</SessionContext.Provider>
	);
}
