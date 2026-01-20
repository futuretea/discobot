"use client";

import { GitCommitHorizontal, PanelRightClose } from "lucide-react";
import { ChatPanel } from "@/components/ide/chat-panel";
import {
	PanelControls,
	type PanelState,
} from "@/components/ide/panel-controls";
import { TerminalView } from "@/components/ide/terminal-view";
import { Button } from "@/components/ui/button";
import { useSessionContext } from "@/lib/contexts/session-context";

type BottomView = "chat" | "terminal";

interface BottomPanelProps {
	panelState: PanelState;
	style: React.CSSProperties;
	showPanelControls: boolean;
	view: BottomView;
	onViewChange: (view: BottomView) => void;
	onMinimize: () => void;
	rightSidebarOpen?: boolean;
	onToggleRightSidebar?: () => void;
	changedFilesCount?: number;
}

export function BottomPanel({
	panelState,
	style,
	showPanelControls,
	view,
	onViewChange,
	onMinimize,
	rightSidebarOpen,
	onToggleRightSidebar,
	changedFilesCount = 0,
}: BottomPanelProps) {
	const { selectedSessionId } = useSessionContext();

	return (
		<div className="flex flex-col overflow-hidden" style={style}>
			{/* Bottom panel header */}
			<div className="h-10 flex items-center justify-between px-2 bg-muted/30 border-b border-border shrink-0">
				<div className="flex items-center gap-2">
					<Button
						variant={view === "chat" ? "secondary" : "ghost"}
						size="sm"
						className="h-6 text-xs"
						onClick={() => onViewChange("chat")}
					>
						Chat
					</Button>
					<Button
						variant={view === "terminal" ? "secondary" : "ghost"}
						size="sm"
						className="h-6 text-xs"
						onClick={() => onViewChange("terminal")}
					>
						Terminal
					</Button>
				</div>
				<div className="flex items-center gap-2">
					{showPanelControls && (
						<PanelControls
							state={panelState}
							onMinimize={onMinimize}
							showMinimize={false}
							showMaximize={false}
						/>
					)}
					{changedFilesCount > 0 && (
						<Button
							variant="default"
							size="sm"
							className="h-6 text-xs gap-1"
							onClick={() => {
								// TODO: Wire up commit functionality
								console.log("Commit clicked");
							}}
							title="Commit changes"
						>
							<GitCommitHorizontal className="h-3.5 w-3.5" />
							Commit
						</Button>
					)}
					{onToggleRightSidebar &&
						(rightSidebarOpen ? (
							<Button
								variant="ghost"
								size="icon"
								className="h-6 w-6"
								onClick={onToggleRightSidebar}
								title="Collapse Files"
							>
								<PanelRightClose className="h-3.5 w-3.5" />
							</Button>
						) : (
							<Button
								variant="ghost"
								size="sm"
								className="h-6 text-xs"
								onClick={onToggleRightSidebar}
							>
								{changedFilesCount > 0
									? `Changes (${changedFilesCount})`
									: "Files"}
							</Button>
						))}
				</div>
			</div>
			{panelState !== "minimized" && (
				<div className="flex-1 overflow-hidden">
					{view === "terminal" ? (
						<TerminalView
							className="h-full"
							onToggleChat={() => onViewChange("chat")}
							hideHeader
						/>
					) : (
						<ChatPanel key={selectedSessionId} className="h-full" />
					)}
				</div>
			)}
		</div>
	);
}
