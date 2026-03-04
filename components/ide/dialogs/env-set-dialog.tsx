import { AlertTriangle, Pencil, Plus, Trash2, X } from "lucide-react";
import * as React from "react";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api-client";
import type { EnvSetInfo, EnvSetWithVars } from "@/lib/api-types";
import { useEnvSets } from "@/lib/hooks/use-env-sets";

interface EnvSetDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

type DialogView = "list" | "form";

interface KVRow {
	id: string;
	key: string;
	value: string;
}

let rowCounter = 0;
function newRow(key = "", value = ""): KVRow {
	return { id: String(rowCounter++), key, value };
}

function EnvSetForm({
	initial,
	onSave,
	onCancel,
}: {
	initial?: EnvSetWithVars;
	onSave: (name: string, envVars: Record<string, string>) => Promise<void>;
	onCancel: () => void;
}) {
	const [name, setName] = React.useState(initial?.name ?? "");
	const [rows, setRows] = React.useState<KVRow[]>(() => {
		if (initial?.envVars) {
			const entries = Object.entries(initial.envVars);
			return entries.length > 0
				? entries.map(([key, value]) => newRow(key, value))
				: [newRow()];
		}
		return [newRow()];
	});
	const [isSaving, setIsSaving] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);

	const addRow = () => setRows((prev) => [...prev, newRow()]);

	const removeRow = (idx: number) =>
		setRows((prev) => prev.filter((_, i) => i !== idx));

	const updateRow = (idx: number, field: "key" | "value", val: string) =>
		setRows((prev) =>
			prev.map((row, i) => (i === idx ? { ...row, [field]: val } : row)),
		);

	const handleSave = async () => {
		if (!name.trim()) {
			setError("Name is required.");
			return;
		}
		const envVars: Record<string, string> = {};
		for (const row of rows) {
			const k = row.key.trim();
			if (k) {
				envVars[k] = row.value;
			}
		}
		setIsSaving(true);
		setError(null);
		try {
			await onSave(name.trim(), envVars);
		} catch {
			setError("Failed to save. Please try again.");
			setIsSaving(false);
		}
	};

	return (
		<div className="space-y-4">
			{/* Name */}
			<div className="space-y-1.5">
				<Label htmlFor="env-set-name">Name</Label>
				<Input
					id="env-set-name"
					value={name}
					onChange={(e) => setName(e.target.value)}
					placeholder="e.g. Production"
					autoFocus
				/>
			</div>

			{/* Key-value editor */}
			<div className="space-y-1.5">
				<Label>Environment variables</Label>
				<div className="space-y-2 max-h-[280px] overflow-y-auto pr-1">
					{rows.map((row, idx) => (
						<div key={row.id} className="flex gap-2 items-center">
							<Input
								value={row.key}
								onChange={(e) => updateRow(idx, "key", e.target.value)}
								placeholder="KEY"
								className="flex-1 font-mono text-xs"
							/>
							<Input
								value={row.value}
								onChange={(e) => updateRow(idx, "value", e.target.value)}
								placeholder="value"
								className="flex-1 font-mono text-xs"
							/>
							<Button
								type="button"
								variant="ghost"
								size="icon"
								className="h-8 w-8 shrink-0 text-muted-foreground hover:text-destructive"
								onClick={() => removeRow(idx)}
								disabled={rows.length === 1}
								title="Remove variable"
							>
								<X className="h-3.5 w-3.5" />
							</Button>
						</div>
					))}
				</div>
				<Button
					type="button"
					variant="ghost"
					size="sm"
					className="text-xs text-muted-foreground gap-1"
					onClick={addRow}
				>
					<Plus className="h-3 w-3" />
					Add variable
				</Button>
			</div>

			{error && <p className="text-sm text-destructive">{error}</p>}

			{/* Actions */}
			<div className="flex gap-2 justify-end pt-2">
				<Button variant="ghost" onClick={onCancel} disabled={isSaving}>
					Cancel
				</Button>
				<Button onClick={handleSave} disabled={isSaving}>
					{isSaving ? "Saving..." : initial ? "Save changes" : "Create"}
				</Button>
			</div>
		</div>
	);
}

export function EnvSetDialog({ open, onOpenChange }: EnvSetDialogProps) {
	const { envSets, isLoading, createEnvSet, updateEnvSet, deleteEnvSet } =
		useEnvSets();

	const [view, setView] = React.useState<DialogView>("list");
	const [editingId, setEditingId] = React.useState<string | null>(null);
	const [editingData, setEditingData] = React.useState<EnvSetWithVars | null>(
		null,
	);

	// Reset on close
	React.useEffect(() => {
		if (!open) {
			setView("list");
			setEditingId(null);
			setEditingData(null);
		}
	}, [open]);

	const handleCreate = async (
		name: string,
		envVars: Record<string, string>,
	) => {
		await createEnvSet(name, envVars);
		setView("list");
	};

	const handleUpdate = async (
		name: string,
		envVars: Record<string, string>,
	) => {
		if (!editingId) return;
		await updateEnvSet(editingId, name, envVars);
		setView("list");
		setEditingId(null);
		setEditingData(null);
	};

	const handleEdit = async (envSet: EnvSetInfo) => {
		try {
			const full = await api.getEnvSet(envSet.id);
			setEditingId(envSet.id);
			setEditingData(full);
			setView("form");
		} catch {
			// If fetch fails, open form with empty vars
			setEditingId(envSet.id);
			setEditingData({ ...envSet, envVars: {} });
			setView("form");
		}
	};

	const handleDelete = async (id: string) => {
		await deleteEnvSet(id);
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-[500px]">
				<DialogHeader>
					<DialogTitle>Env Sets</DialogTitle>
					<DialogDescription>
						Create named collections of environment variables to inject into
						agent sessions.
					</DialogDescription>
				</DialogHeader>

				<div className="flex gap-2 rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-2.5 text-xs text-amber-600 dark:text-amber-400">
					<AlertTriangle className="h-3.5 w-3.5 shrink-0 mt-0.5" />
					<span>
						The agent will have full read access to all values in any active env
						set. Only include variables you are comfortable sharing with the
						agent.
					</span>
				</div>

				{view === "form" ? (
					<EnvSetForm
						initial={editingData ?? undefined}
						onSave={editingId ? handleUpdate : handleCreate}
						onCancel={() => {
							setView("list");
							setEditingId(null);
							setEditingData(null);
						}}
					/>
				) : (
					<div className="space-y-4">
						{/* List */}
						{isLoading && (
							<div className="text-center py-6 text-muted-foreground text-sm">
								Loading...
							</div>
						)}

						{!isLoading && envSets.length === 0 && (
							<div className="text-center py-6 text-muted-foreground text-sm">
								No env sets yet. Create one to get started.
							</div>
						)}

						{!isLoading && envSets.length > 0 && (
							<div className="space-y-1">
								{envSets.map((envSet) => (
									<div
										key={envSet.id}
										className="flex items-center justify-between gap-2 px-3 py-2 rounded-md hover:bg-muted/50 group"
									>
										<span className="text-sm font-medium truncate">
											{envSet.name}
										</span>
										<div className="flex gap-1 shrink-0">
											<Button
												variant="ghost"
												size="icon"
												className="h-7 w-7 text-muted-foreground hover:text-foreground opacity-0 group-hover:opacity-100"
												onClick={() => handleEdit(envSet)}
												title="Edit"
											>
												<Pencil className="h-3.5 w-3.5" />
											</Button>
											<Button
												variant="ghost"
												size="icon"
												className="h-7 w-7 text-muted-foreground hover:text-destructive opacity-0 group-hover:opacity-100"
												onClick={() => handleDelete(envSet.id)}
												title="Delete"
											>
												<Trash2 className="h-3.5 w-3.5" />
											</Button>
										</div>
									</div>
								))}
							</div>
						)}

						{/* Create button */}
						<Button
							variant="outline"
							className="w-full justify-start gap-2"
							onClick={() => {
								setEditingId(null);
								setEditingData(null);
								setView("form");
							}}
						>
							<Plus className="h-4 w-4" />
							Create env set
						</Button>
					</div>
				)}
			</DialogContent>
		</Dialog>
	);
}
