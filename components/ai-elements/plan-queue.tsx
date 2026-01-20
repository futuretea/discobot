"use client";

import {
	CheckCircle2,
	ChevronDown,
	ChevronRight,
	Circle,
	ListTodo,
	Loader2,
} from "lucide-react";
import {
	Collapsible,
	CollapsibleContent,
	CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";

// Plan entry structure from ACP
export interface PlanEntry {
	content: string;
	status: "pending" | "in_progress" | "completed";
	priority?: "low" | "medium" | "high";
}

interface PlanQueueProps {
	entries: PlanEntry[];
	isOpen?: boolean;
	onOpenChange?: (open: boolean) => void;
	className?: string;
}

// Get status icon based on entry status
function getStatusIcon(status: PlanEntry["status"]) {
	switch (status) {
		case "completed":
			return <CheckCircle2 className="h-4 w-4 text-green-500 shrink-0" />;
		case "in_progress":
			return (
				<Loader2 className="h-4 w-4 text-blue-500 animate-spin shrink-0" />
			);
		default:
			return <Circle className="h-4 w-4 text-muted-foreground shrink-0" />;
	}
}

export function PlanQueue({
	entries,
	isOpen = true,
	onOpenChange,
	className,
}: PlanQueueProps) {
	// Calculate completion stats
	const completedCount = entries.filter((e) => e.status === "completed").length;
	const inProgressCount = entries.filter(
		(e) => e.status === "in_progress",
	).length;
	const totalCount = entries.length;

	// Format summary text
	const getSummaryText = () => {
		if (totalCount === 0) return "No items";
		if (completedCount === totalCount) return `${totalCount} completed`;
		if (inProgressCount > 0) {
			return `${completedCount}/${totalCount} done, ${inProgressCount} in progress`;
		}
		return `${completedCount}/${totalCount} done`;
	};

	return (
		<Collapsible
			open={isOpen}
			onOpenChange={onOpenChange}
			className={cn("border-t border-border bg-muted/30", className)}
		>
			<CollapsibleTrigger className="flex w-full items-center gap-2 px-4 py-2 text-left hover:bg-muted/50 transition-colors">
				{isOpen ? (
					<ChevronDown className="h-4 w-4 text-muted-foreground shrink-0" />
				) : (
					<ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />
				)}
				<ListTodo className="h-4 w-4 text-muted-foreground shrink-0" />
				<span className="text-sm font-medium">Plan</span>
				<span className="text-xs text-muted-foreground ml-auto">
					{getSummaryText()}
				</span>
			</CollapsibleTrigger>
			<CollapsibleContent>
				<div className="px-4 pb-3 space-y-1">
					{entries.map((entry, index) => (
						<div
							// biome-ignore lint/suspicious/noArrayIndexKey: Plan entries don't have unique IDs
							key={index}
							className={cn(
								"flex items-start gap-2 py-1.5 px-2 rounded-md text-sm",
								entry.status === "completed" && "text-muted-foreground",
								entry.status === "in_progress" && "bg-blue-500/10",
							)}
						>
							{getStatusIcon(entry.status)}
							<span
								className={cn(
									"flex-1",
									entry.status === "completed" && "line-through",
								)}
							>
								{entry.content}
							</span>
						</div>
					))}
					{entries.length === 0 && (
						<div className="text-sm text-muted-foreground text-center py-2">
							No plan items yet
						</div>
					)}
				</div>
			</CollapsibleContent>
		</Collapsible>
	);
}
