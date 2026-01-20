"use client";

import * as React from "react";
import { AgentsPanel } from "@/components/ide/agents-panel";
import { ResizeHandle } from "@/components/ide/resize-handle";
import { SidebarTree } from "@/components/ide/sidebar-tree";

interface LeftSidebarProps {
	isOpen: boolean;
	width: number;
	onResize: (delta: number) => void;
}

export function LeftSidebar({ isOpen, width, onResize }: LeftSidebarProps) {
	const [agentsPanelMinimized, setAgentsPanelMinimized] = React.useState(false);
	const [agentsPanelHeight, setAgentsPanelHeight] = React.useState(20);
	const sidebarRef = React.useRef<HTMLDivElement>(null);

	const handleAgentsPanelResize = React.useCallback((delta: number) => {
		if (!sidebarRef.current) return;
		const containerHeight = sidebarRef.current.clientHeight;
		const deltaPercent = (delta / containerHeight) * 100;
		setAgentsPanelHeight((prev) =>
			Math.min(60, Math.max(15, prev - deltaPercent)),
		);
	}, []);

	if (!isOpen) {
		return null;
	}

	return (
		<div className="flex">
			<aside
				ref={sidebarRef}
				className="border-r border-border bg-sidebar overflow-hidden flex flex-col"
				style={{ width }}
			>
				{/* Workspaces section - takes remaining space */}
				<SidebarTree className="flex-1 min-h-0" />

				{/* Resize handle between workspaces and agents */}
				{!agentsPanelMinimized && (
					<ResizeHandle onResize={handleAgentsPanelResize} />
				)}

				{/* Agents section - 20% default */}
				<AgentsPanel
					isMinimized={agentsPanelMinimized}
					onToggleMinimize={() =>
						setAgentsPanelMinimized(!agentsPanelMinimized)
					}
					style={
						agentsPanelMinimized ? {} : { height: `${agentsPanelHeight}%` }
					}
				/>
			</aside>

			{/* Vertical resize handle for sidebar width */}
			<ResizeHandle orientation="vertical" onResize={onResize} />
		</div>
	);
}
