import { Hammer, Map as MapIcon } from "lucide-react";
import * as React from "react";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

interface ModeSelectorProps {
	selectedMode: string | null; // "plan" or null (null = build/default)
	onSelectMode: (mode: string | null) => void;
	disabled?: boolean;
	/** Compact mode for chat input (icon button) */
	compact?: boolean;
}

const modes = [
	{
		id: null,
		label: "Build",
		description: "Execute code, edit files, run tools",
		icon: Hammer,
	},
	{
		id: "plan",
		label: "Plan",
		description: "Plan only, no tool execution",
		icon: MapIcon,
	},
] as const;

export function ModeSelector({
	selectedMode,
	onSelectMode,
	disabled = false,
	compact = false,
}: ModeSelectorProps) {
	const selected = modes.find((m) => m.id === selectedMode) ?? modes[0];
	const Icon = selected.icon;

	return (
		<DropdownMenu>
			<DropdownMenuTrigger asChild>
				<Button
					variant="ghost"
					size="sm"
					className={
						compact
							? "h-8 shrink-0 px-2 text-xs text-muted-foreground hover:text-foreground"
							: "gap-2 min-w-[140px] justify-between"
					}
					disabled={disabled}
					title={`Mode: ${selected.label}`}
				>
					<Icon className="h-3 w-3 shrink-0" />
					<span className="truncate">{selected.label}</span>
				</Button>
			</DropdownMenuTrigger>
			<DropdownMenuContent align="center" className="w-[240px]">
				{modes.map((mode) => {
					const ModeIcon = mode.icon;
					return (
						<DropdownMenuItem
							key={mode.id ?? "build"}
							onClick={() => onSelectMode(mode.id)}
							className="flex items-start gap-2"
						>
							<ModeIcon className="h-4 w-4 mt-0.5 shrink-0" />
							<div className="flex flex-col gap-0.5">
								<span className="font-medium">{mode.label}</span>
								<span className="text-xs text-muted-foreground">
									{mode.description}
								</span>
							</div>
						</DropdownMenuItem>
					);
				})}
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
