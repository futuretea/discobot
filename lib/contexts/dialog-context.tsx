import * as React from "react";
import { api } from "@/lib/api-client";
import type {
	Agent,
	CreateAgentRequest,
	CreateWorkspaceRequest,
	StatusMessage,
	Workspace,
} from "@/lib/api-types";
import { useAgentTypes } from "@/lib/hooks/use-agent-types";
import { useAgents } from "@/lib/hooks/use-agents";
import { useAuthProviders } from "@/lib/hooks/use-auth-providers";
import { useCredentials } from "@/lib/hooks/use-credentials";
import {
	type DialogControl,
	useDialogControl,
} from "@/lib/hooks/use-dialog-control";
import { useWorkspaces } from "@/lib/hooks/use-workspaces";
import { useMainContentContext } from "./main-content-context";

// Dialog data types
interface AgentDialogData {
	agent?: Agent;
	agentTypeId?: string;
}

interface WorkspaceDialogData {
	mode?: "git" | "local" | "generic";
}

interface CredentialsDialogData {
	providerId?: string;
}

interface DialogContextValue {
	// Dialog controls
	workspaceDialog: DialogControl<WorkspaceDialogData>;
	agentDialog: DialogControl<AgentDialogData>;
	deleteWorkspaceDialog: DialogControl<Workspace>;
	credentialsDialog: DialogControl<CredentialsDialogData>;
	skillsDialog: DialogControl<Record<string, never>>;
	mcpServersDialog: DialogControl<Record<string, never>>;
	supportInfoDialog: DialogControl<Record<string, never>>;

	// System requirements (special case - driven by API response)
	systemRequirements: {
		isOpen: boolean;
		messages: StatusMessage[];
		close: () => void;
	};

	// Action handlers
	handleAddWorkspace: (data: CreateWorkspaceRequest) => Promise<void>;
	handleAddOrEditAgent: (data: CreateAgentRequest) => Promise<void>;
	handleConfirmDeleteWorkspace: (deleteFiles: boolean) => Promise<void>;

	// Data for dialogs
	authProviders: ReturnType<typeof useAuthProviders>["authProviders"];
	credentials: ReturnType<typeof useCredentials>["credentials"];
}

const DialogContext = React.createContext<DialogContextValue | null>(null);

export function useDialogContext() {
	const context = React.useContext(DialogContext);
	if (!context) {
		throw new Error("useDialogContext must be used within a DialogProvider");
	}
	return context;
}

interface DialogProviderProps {
	children: React.ReactNode;
}

export function DialogProvider({ children }: DialogProviderProps) {
	const mainPanel = useMainContentContext();
	const workspace = useWorkspaces();
	const { createAgent, updateAgent } = useAgents();
	useAgentTypes(); // Preload agent types for dialog
	const { authProviders } = useAuthProviders();
	const { credentials } = useCredentials();

	// Dialog controls using the generic hook
	const workspaceDialog = useDialogControl<WorkspaceDialogData>();
	const agentDialog = useDialogControl<AgentDialogData>();
	const deleteWorkspaceDialog = useDialogControl<Workspace>();
	const credentialsDialog = useDialogControl<CredentialsDialogData>();
	const skillsDialog = useDialogControl<Record<string, never>>();
	const mcpServersDialog = useDialogControl<Record<string, never>>();
	const supportInfoDialog = useDialogControl<Record<string, never>>();

	// System status state (special case - populated by API)
	const [systemStatusMessages, setSystemStatusMessages] = React.useState<
		StatusMessage[]
	>([]);
	const [showSystemRequirements, setShowSystemRequirements] =
		React.useState(false);

	// Check system status on mount
	React.useEffect(() => {
		async function checkSystemStatus() {
			try {
				const status = await api.getSystemStatus();
				if (status.messages && status.messages.length > 0) {
					setSystemStatusMessages(status.messages);
					setShowSystemRequirements(true);
				}
			} catch (error) {
				console.error("Failed to check system status:", error);
			}
		}
		checkSystemStatus();
	}, []);

	// Action handlers
	const handleAddWorkspace = React.useCallback(
		async (data: CreateWorkspaceRequest) => {
			const ws = await workspace.createWorkspace(data);
			// Dialog will close itself on success
			if (ws) {
				mainPanel.showNewSession({ workspaceId: ws.id });
			}
		},
		[workspace, mainPanel],
	);

	const handleAddOrEditAgent = React.useCallback(
		async (agentData: CreateAgentRequest) => {
			if (agentData.id) {
				await updateAgent(agentData.id, agentData);
			} else {
				await createAgent(agentData);
			}
			agentDialog.close();
		},
		[agentDialog, createAgent, updateAgent],
	);

	const handleConfirmDeleteWorkspace = React.useCallback(
		async (deleteFiles: boolean) => {
			const ws = deleteWorkspaceDialog.data;
			if (!ws) return;

			await workspace.deleteWorkspace(ws.id, deleteFiles);

			// Check if current view is related to the deleted workspace
			const { view, selectedSession, showNewSession } = mainPanel;

			// Clear selection if viewing a session from the deleted workspace
			if (selectedSession?.workspaceId === ws.id) {
				showNewSession();
			}
			// Clear selection if new-session view has the deleted workspace preselected
			else if (view.type === "new-session" && view.workspaceId === ws.id) {
				showNewSession();
			}

			deleteWorkspaceDialog.close();
		},
		[deleteWorkspaceDialog, workspace, mainPanel],
	);

	const closeSystemRequirements = React.useCallback(() => {
		setShowSystemRequirements(false);
	}, []);

	const value = React.useMemo<DialogContextValue>(
		() => ({
			// Dialog controls
			workspaceDialog,
			agentDialog,
			deleteWorkspaceDialog,
			credentialsDialog,
			skillsDialog,
			mcpServersDialog,
			supportInfoDialog,

			// System requirements
			systemRequirements: {
				isOpen: showSystemRequirements,
				messages: systemStatusMessages,
				close: closeSystemRequirements,
			},

			// Action handlers
			handleAddWorkspace,
			handleAddOrEditAgent,
			handleConfirmDeleteWorkspace,

			// Data
			authProviders,
			credentials,
		}),
		[
			workspaceDialog,
			agentDialog,
			deleteWorkspaceDialog,
			credentialsDialog,
			skillsDialog,
			mcpServersDialog,
			supportInfoDialog,
			showSystemRequirements,
			systemStatusMessages,
			closeSystemRequirements,
			handleAddWorkspace,
			handleAddOrEditAgent,
			handleConfirmDeleteWorkspace,
			authProviders,
			credentials,
		],
	);

	return (
		<DialogContext.Provider value={value}>{children}</DialogContext.Provider>
	);
}
