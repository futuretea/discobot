import { Check, Layers, Settings } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { EnvSetInfo } from "@/lib/api-types";

interface EnvSetSelectorProps {
	activeEnvSetIds: string[];
	envSets: EnvSetInfo[];
	onToggleEnvSet: (id: string) => void;
	onManage: () => void;
}

export function EnvSetSelector({
	activeEnvSetIds,
	envSets,
	onToggleEnvSet,
	onManage,
}: EnvSetSelectorProps) {
	const activeCount = activeEnvSetIds.length;
	const activeNames = activeEnvSetIds
		.map((id) => envSets.find((e) => e.id === id)?.name)
		.filter(Boolean)
		.join(", ");

	const label =
		activeCount === 0
			? "Env set"
			: activeCount === 1
				? (envSets.find((e) => e.id === activeEnvSetIds[0])?.name ?? "Env set")
				: `${activeCount} env sets`;

	const title =
		activeCount === 0
			? "No environment sets active"
			: `Env sets: ${activeNames}`;

	return (
		<DropdownMenu>
			<DropdownMenuTrigger asChild>
				<Button
					variant="ghost"
					size="sm"
					className="h-8 shrink-0 px-2 text-xs text-muted-foreground hover:text-foreground gap-1.5"
					title={title}
				>
					<Layers
						className={`h-3.5 w-3.5 shrink-0 ${activeCount > 0 ? "text-amber-500" : ""}`}
					/>
					{activeCount > 0 && (
						<span className="truncate max-w-[120px]">{label}</span>
					)}
				</Button>
			</DropdownMenuTrigger>
			<DropdownMenuContent align="center" className="w-[220px]">
				{/* Env set list */}
				{envSets.map((envSet) => {
					const isActive = activeEnvSetIds.includes(envSet.id);
					return (
						<DropdownMenuItem
							key={envSet.id}
							onClick={() => onToggleEnvSet(envSet.id)}
							className="flex items-center gap-2"
						>
							{isActive ? (
								<Check className="h-3.5 w-3.5 shrink-0" />
							) : (
								<span className="w-3.5 shrink-0" />
							)}
							<span className="truncate">{envSet.name}</span>
						</DropdownMenuItem>
					);
				})}

				{envSets.length > 0 && <DropdownMenuSeparator />}

				{/* Manage link */}
				<DropdownMenuItem
					onClick={onManage}
					className="flex items-center gap-2 text-muted-foreground"
				>
					<Settings className="h-3.5 w-3.5 shrink-0" />
					<span>Manage env sets...</span>
				</DropdownMenuItem>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
