"use client";

import type * as React from "react";
import { AgentProvider } from "./agent-context";
import { MainPanelProvider } from "./main-panel-context";
import { ProjectEventsProvider } from "./project-events-context";
import { SessionProvider } from "./session-context";

interface AppProviderProps {
	children: React.ReactNode;
}

/**
 * Combined provider that wraps all domain contexts.
 * - ProjectEventsProvider: SSE connection for real-time updates
 * - AgentProvider: Agent and SupportedAgentType objects
 * - MainPanelProvider: Main panel view state (what's displayed in the center)
 * - SessionProvider: Session objects (now derives state from MainPanelProvider)
 */
export function AppProvider({ children }: AppProviderProps) {
	return (
		<ProjectEventsProvider>
			<AgentProvider>
				<MainPanelProvider>
					<SessionProvider>{children}</SessionProvider>
				</MainPanelProvider>
			</AgentProvider>
		</ProjectEventsProvider>
	);
}
