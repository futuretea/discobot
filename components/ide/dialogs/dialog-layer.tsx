"use client";

import dynamic from "next/dynamic";
import { useAgentContext } from "@/lib/contexts/agent-context";
import { useDialogContext } from "@/lib/contexts/dialog-context";
import { useWorkspaces } from "@/lib/hooks/use-workspaces";

// Dynamic imports for dialogs - not needed on initial render
const AddWorkspaceDialog = dynamic(
	() => import("./add-workspace-dialog").then((m) => m.AddWorkspaceDialog),
	{ ssr: false },
);

const AddAgentDialog = dynamic(
	() => import("./add-agent-dialog").then((m) => m.AddAgentDialog),
	{ ssr: false },
);

const DeleteWorkspaceDialog = dynamic(
	() =>
		import("./delete-workspace-dialog").then((m) => m.DeleteWorkspaceDialog),
	{ ssr: false },
);

const CredentialsDialog = dynamic(
	() => import("./credentials-dialog").then((m) => m.CredentialsDialog),
	{ ssr: false },
);

const SystemRequirementsDialog = dynamic(
	() =>
		import("./system-requirements-dialog").then(
			(m) => m.SystemRequirementsDialog,
		),
	{ ssr: false },
);

const WelcomeModal = dynamic(
	() => import("./welcome-modal").then((m) => m.WelcomeModal),
	{ ssr: false },
);

export function DialogLayer() {
	const { workspaces } = useWorkspaces();
	const { agents, agentTypes, isLoading: agentsLoading } = useAgentContext();
	const dialogs = useDialogContext();

	return (
		<>
			<AddWorkspaceDialog
				open={dialogs.workspaceDialog.isOpen}
				onOpenChange={dialogs.workspaceDialog.onOpenChange}
				onAdd={dialogs.handleAddWorkspace}
			/>

			<AddAgentDialog
				open={dialogs.agentDialog.isOpen}
				onOpenChange={dialogs.agentDialog.onOpenChange}
				onAdd={dialogs.handleAddOrEditAgent}
				editingAgent={dialogs.agentDialog.data?.agent}
				onOpenCredentials={(providerId) =>
					dialogs.credentialsDialog.open({ providerId })
				}
				preselectedAgentTypeId={dialogs.agentDialog.data?.agentTypeId}
			/>

			<DeleteWorkspaceDialog
				open={dialogs.deleteWorkspaceDialog.isOpen}
				onOpenChange={dialogs.deleteWorkspaceDialog.onOpenChange}
				workspace={dialogs.deleteWorkspaceDialog.data}
				onConfirm={dialogs.handleConfirmDeleteWorkspace}
			/>

			<CredentialsDialog
				open={dialogs.credentialsDialog.isOpen}
				onOpenChange={dialogs.credentialsDialog.onOpenChange}
				initialProviderId={dialogs.credentialsDialog.data?.providerId}
			/>

			<SystemRequirementsDialog
				open={dialogs.systemRequirements.isOpen}
				messages={dialogs.systemRequirements.messages}
				onClose={dialogs.systemRequirements.close}
			/>

			<WelcomeModal
				open={
					dialogs.welcome.systemStatusChecked &&
					!dialogs.systemRequirements.isOpen &&
					!agentsLoading &&
					agents.length === 0 &&
					!dialogs.welcome.skipped
				}
				agentTypes={agentTypes}
				authProviders={dialogs.authProviders}
				configuredCredentials={dialogs.credentials}
				hasExistingWorkspaces={workspaces.length > 0}
				onSkip={() => dialogs.welcome.setSkipped(true)}
				onComplete={dialogs.handleWelcomeComplete}
			/>
		</>
	);
}
