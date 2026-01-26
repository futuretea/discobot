"use client";

import type * as React from "react";
import { AgentProvider } from "./agent-context";
import { ProjectEventsProvider } from "./project-events-context";
import { SessionProvider } from "./session-context";

interface AppProviderProps {
	children: React.ReactNode;
}

/**
 * Combined provider that wraps all domain contexts.
 * - ProjectEventsProvider: SSE connection for real-time updates
 * - AgentProvider: Agent and SupportedAgentType objects
 * - SessionProvider: Session objects and selection state
 */
export function AppProvider({ children }: AppProviderProps) {
	return (
		<ProjectEventsProvider>
			<AgentProvider>
				<SessionProvider>{children}</SessionProvider>
			</AgentProvider>
		</ProjectEventsProvider>
	);
}
