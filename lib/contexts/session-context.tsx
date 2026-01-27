"use client";

import * as React from "react";
import useSWR from "swr";
import { api } from "@/lib/api-client";
import type { Session } from "@/lib/api-types";
import { useMainPanelContext } from "./main-panel-context";

export interface SessionContextValue {
	// Data
	selectedSessionId: string | null;
	selectedSession: Session | null | undefined;

	// UI state for session creation flow
	preselectedWorkspaceId: string | null;
	preselectedAgentId: string | null;
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
	const { view, showSession } = useMainPanelContext();

	// Derive state from MainPanelContext
	const selectedSessionId = view.type === "session" ? view.sessionId : null;
	const preselectedWorkspaceId =
		view.type === "new-session" ? (view.workspaceId ?? null) : null;
	const preselectedAgentId =
		view.type === "new-session" ? (view.agentId ?? null) : null;

	const [workspaceSelectTrigger, setWorkspaceSelectTrigger] = React.useState(0);
	const [chatResetTrigger, setChatResetTrigger] = React.useState(0);

	// Fetch the selected session via SWR
	const { data: selectedSession, mutate: mutateSelectedSession } = useSWR(
		selectedSessionId ? `session-${selectedSessionId}` : null,
		() => (selectedSessionId ? api.getSession(selectedSessionId) : null),
	);

	// Sync triggers when view changes to new-session with preselected workspace
	React.useEffect(() => {
		if (view.type === "new-session" && view.workspaceId) {
			setWorkspaceSelectTrigger((prev) => prev + 1);
		}
	}, [view]);

	// Actions - these now delegate to MainPanelContext
	const selectSession = React.useCallback(
		(sessionId: string | null) => {
			if (sessionId) {
				showSession(sessionId);
			}
		},
		[showSession],
	);

	const handleSessionSelect = React.useCallback(
		(session: { id: string }) => {
			showSession(session.id);
		},
		[showSession],
	);

	const handleNewSession = React.useCallback(() => {
		// This is handled by Header using showNewSession directly
		setChatResetTrigger((prev) => prev + 1);
	}, []);

	const handleAddSession = React.useCallback((_workspaceId: string) => {
		// This is handled by SessionListTable using showNewSession directly
		setWorkspaceSelectTrigger((prev) => prev + 1);
	}, []);

	const handleSessionCreated = React.useCallback(
		(sessionId: string) => {
			showSession(sessionId);
		},
		[showSession],
	);

	const value = React.useMemo<SessionContextValue>(
		() => ({
			selectedSessionId,
			selectedSession: selectedSession ?? null,
			preselectedWorkspaceId,
			preselectedAgentId,
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
			preselectedAgentId,
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
