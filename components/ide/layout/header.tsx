"use client";

import {
	Bot,
	Check,
	ChevronDown,
	Key,
	MessageSquare,
	PanelLeft,
	PanelLeftClose,
	Plus,
	Trash2,
	X,
} from "lucide-react";
import * as React from "react";
import { CredentialsDialog } from "@/components/ide/credentials-dialog";
import { IconRenderer } from "@/components/ide/icon-renderer";
import { OctobotLogo } from "@/components/ide/octobot-logo";
import { ThemeToggle } from "@/components/ide/theme-toggle";
import { isTauriEnv, WindowControls } from "@/components/ide/window-controls";
import {
	getWorkspaceDisplayPath,
	WorkspaceIcon,
} from "@/components/ide/workspace-path";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { api } from "@/lib/api-client";
import type { Agent, Workspace } from "@/lib/api-types";
import { useSessionContext } from "@/lib/contexts/session-context";
import { useDeleteSession, useSessions } from "@/lib/hooks/use-sessions";
import { formatTimeAgo } from "@/lib/utils";

interface HeaderProps {
	leftSidebarOpen: boolean;
	onToggleSidebar: () => void;
	onNewSession: () => void;
}

export function Header({
	leftSidebarOpen,
	onToggleSidebar,
	onNewSession,
}: HeaderProps) {
	const {
		workspaces,
		agentTypes,
		selectedSession,
		sessionAgent,
		sessionWorkspace,
		handleSessionSelect,
	} = useSessionContext();

	const [credentialsOpen, setCredentialsOpen] = React.useState(false);
	const [confirmDeleteId, setConfirmDeleteId] = React.useState<string | null>(
		null,
	);

	const getAgentIcons = (a: Agent) => {
		const agentType = agentTypes.find((t) => t.id === a.agentType);
		return agentType?.icons;
	};

	// Fetch sessions for current workspace via SWR
	const { sessions: workspaceSessions } = useSessions(
		sessionWorkspace?.id ?? null,
	);

	const hasSession = selectedSession || sessionWorkspace;

	// Handle workspace selection from breadcrumb dropdown
	const handleWorkspaceSelect = React.useCallback(
		async (workspace: Workspace) => {
			// Fetch sessions for this workspace and select the first one
			try {
				const { sessions } = await api.getSessions(workspace.id);
				const firstSession = sessions[0];
				if (firstSession) {
					handleSessionSelect(firstSession);
				}
			} catch (error) {
				console.error("Failed to fetch sessions for workspace:", error);
			}
		},
		[handleSessionSelect],
	);

	// Handle session deletion with inline confirmation
	const { deleteSession } = useDeleteSession();
	const handleDeleteClick = React.useCallback(
		(e: React.MouseEvent, sessionId: string) => {
			e.stopPropagation();
			setConfirmDeleteId(sessionId);
		},
		[],
	);
	const handleConfirmDelete = React.useCallback(
		async (e: React.MouseEvent, sessionId: string) => {
			e.stopPropagation();
			const isCurrentSession = selectedSession?.id === sessionId;
			await deleteSession(sessionId);
			setConfirmDeleteId(null);
			if (isCurrentSession) {
				onNewSession();
			}
		},
		[deleteSession, selectedSession?.id, onNewSession],
	);
	const handleCancelDelete = React.useCallback((e: React.MouseEvent) => {
		e.stopPropagation();
		setConfirmDeleteId(null);
	}, []);

	// Detect macOS for window control placement
	const [isMac, setIsMac] = React.useState(false);
	React.useEffect(() => {
		if (!isTauriEnv) return;
		import("@tauri-apps/plugin-os").then(({ platform }) => {
			setIsMac(platform() === "macos");
		});
	}, []);

	return (
		<header className="h-12 border-b border-border flex items-center justify-between px-4 relative z-[60] bg-background">
			{/* Drag region layer - covers header but behind content */}
			<div
				className="absolute inset-0 pointer-events-auto"
				data-tauri-drag-region
			/>
			<div className="flex items-center gap-2 min-w-0 relative">
				{/* macOS window controls on the left */}
				{isTauriEnv && isMac && <WindowControls />}
				<Button
					variant="ghost"
					size="icon"
					onClick={onToggleSidebar}
					className="tauri-no-drag"
				>
					{leftSidebarOpen ? (
						<PanelLeftClose className="h-4 w-4" />
					) : (
						<PanelLeft className="h-4 w-4" />
					)}
				</Button>
				<div className="flex items-center gap-1.5 shrink-0">
					<OctobotLogo size={22} className="text-primary" />
					<span className="font-semibold">Octobot</span>
				</div>
				<Button
					variant="ghost"
					size="sm"
					className="gap-1.5 text-muted-foreground shrink-0 tauri-no-drag"
					onClick={onNewSession}
				>
					<Plus className="h-4 w-4" />
					New Session
				</Button>

				{/* Breadcrumbs with dropdowns */}
				{hasSession && (
					<>
						<span className="text-muted-foreground shrink-0">/</span>

						{/* Workspace dropdown */}
						{sessionWorkspace && (
							<DropdownMenu>
								<DropdownMenuTrigger asChild>
									<button
										type="button"
										className="flex items-center gap-1.5 text-sm px-2 py-1 rounded-md hover:bg-accent transition-colors min-w-0 tauri-no-drag"
									>
										<WorkspaceIcon
											path={sessionWorkspace.path}
											className="h-4 w-4 shrink-0"
										/>
										<span
											className="truncate max-w-[150px]"
											title={sessionWorkspace.path}
										>
											{getWorkspaceDisplayPath(
												sessionWorkspace.path,
												sessionWorkspace.sourceType,
											)}
										</span>
										<ChevronDown className="h-3 w-3 shrink-0 text-muted-foreground" />
									</button>
								</DropdownMenuTrigger>
								<DropdownMenuContent align="start" className="w-64">
									{workspaces.map((ws) => {
										const isSelected = ws.id === sessionWorkspace.id;
										return (
											<DropdownMenuItem
												key={ws.id}
												onClick={() => handleWorkspaceSelect(ws)}
												className="flex items-center gap-2"
											>
												<WorkspaceIcon
													path={ws.path}
													className="h-4 w-4 shrink-0"
												/>
												<span className="truncate flex-1" title={ws.path}>
													{getWorkspaceDisplayPath(ws.path, ws.sourceType)}
												</span>
												{isSelected && (
													<Check className="h-4 w-4 shrink-0 text-primary" />
												)}
											</DropdownMenuItem>
										);
									})}
								</DropdownMenuContent>
							</DropdownMenu>
						)}

						{/* Session dropdown */}
						{selectedSession && sessionWorkspace && (
							<>
								<span className="text-muted-foreground shrink-0">/</span>
								<DropdownMenu
									onOpenChange={(open) => !open && setConfirmDeleteId(null)}
								>
									<DropdownMenuTrigger asChild>
										<button
											type="button"
											className="flex items-center gap-1.5 text-sm px-2 py-1 rounded-md hover:bg-accent transition-colors min-w-0 tauri-no-drag"
										>
											<MessageSquare className="h-4 w-4 shrink-0 text-muted-foreground" />
											<span className="truncate max-w-[200px] font-medium">
												{selectedSession.name}
											</span>
											<ChevronDown className="h-3 w-3 shrink-0 text-muted-foreground" />
										</button>
									</DropdownMenuTrigger>
									<DropdownMenuContent align="start" className="w-72">
										{workspaceSessions.length > 0 ? (
											workspaceSessions.map((session) => {
												const isSelected = session.id === selectedSession.id;
												const isConfirming = confirmDeleteId === session.id;
												return (
													<DropdownMenuItem
														key={session.id}
														onClick={() => handleSessionSelect(session)}
														className="group/item flex items-center gap-2"
													>
														<MessageSquare className="h-4 w-4 shrink-0 text-muted-foreground" />
														<div className="flex-1 min-w-0">
															<div className="truncate font-medium">
																{session.name}
															</div>
															<div className="text-xs text-muted-foreground truncate">
																{formatTimeAgo(session.timestamp)}
															</div>
														</div>
														{isSelected && !isConfirming && (
															<Check className="h-4 w-4 shrink-0 text-primary" />
														)}
														{isConfirming ? (
															<div className="flex items-center gap-0.5 shrink-0">
																<button
																	type="button"
																	onClick={(e) =>
																		handleConfirmDelete(e, session.id)
																	}
																	className="h-6 w-6 rounded hover:bg-destructive/10 text-destructive flex items-center justify-center"
																	title="Confirm delete"
																>
																	<Check className="h-3.5 w-3.5" />
																</button>
																<button
																	type="button"
																	onClick={handleCancelDelete}
																	className="h-6 w-6 rounded hover:bg-muted flex items-center justify-center"
																	title="Cancel"
																>
																	<X className="h-3.5 w-3.5" />
																</button>
															</div>
														) : (
															<button
																type="button"
																onClick={(e) =>
																	handleDeleteClick(e, session.id)
																}
																className="h-6 w-6 shrink-0 rounded hover:bg-destructive/10 hover:text-destructive flex items-center justify-center opacity-0 group-hover/item:opacity-100 transition-opacity"
																title="Delete session"
															>
																<Trash2 className="h-3.5 w-3.5" />
															</button>
														)}
													</DropdownMenuItem>
												);
											})
										) : (
											<div className="px-2 py-4 text-sm text-muted-foreground text-center">
												No open sessions
											</div>
										)}
										<DropdownMenuSeparator />
										<DropdownMenuItem
											onClick={onNewSession}
											className="flex items-center gap-2"
										>
											<Plus className="h-4 w-4 shrink-0" />
											<span>New Session</span>
										</DropdownMenuItem>
									</DropdownMenuContent>
								</DropdownMenu>
							</>
						)}

						{/* Agent badge (non-interactive) */}
						{sessionAgent && (
							<>
								<span className="text-muted-foreground shrink-0">/</span>
								<div className="flex items-center gap-1.5 text-sm text-muted-foreground">
									{getAgentIcons(sessionAgent) ? (
										<IconRenderer
											icons={getAgentIcons(sessionAgent)}
											size={16}
											className="shrink-0"
										/>
									) : (
										<Bot className="h-4 w-4 shrink-0" />
									)}
									<span className="truncate">{sessionAgent.name}</span>
								</div>
							</>
						)}
					</>
				)}
			</div>
			<div className="flex items-center gap-1 shrink-0 relative">
				<Button
					variant="ghost"
					size="icon"
					onClick={() => setCredentialsOpen(true)}
					title="API Credentials"
					className="tauri-no-drag"
				>
					<Key className="h-4 w-4" />
					<span className="sr-only">API Credentials</span>
				</Button>
				<ThemeToggle className="tauri-no-drag" />
				{/* Windows/Linux window controls on the right */}
				{isTauriEnv && !isMac && <WindowControls />}
			</div>

			<CredentialsDialog
				open={credentialsOpen}
				onOpenChange={setCredentialsOpen}
			/>
		</header>
	);
}
