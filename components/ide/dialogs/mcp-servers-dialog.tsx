import { useMemo, useState } from "react";
import { Globe2, Plus, Server, Terminal, Trash2 } from "lucide-react";

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
import { Textarea } from "@/components/ui/textarea";
import type { MCPServer } from "@/lib/api-types";
import { useMCPServers } from "@/lib/hooks/use-mcp-servers";
import { cn } from "@/lib/utils";

interface MCPServersDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

interface MCPFormState {
	name: string;
	description: string;
	type: "stdio" | "http";
	command: string;
	args: string;
	env: string;
	url: string;
	headers: string;
}

function createEmptyFormState(): MCPFormState {
	return {
		name: "",
		description: "",
		type: "stdio",
		command: "",
		args: "",
		env: "",
		url: "",
		headers: "",
	};
}

/** Splits a textarea value (newline- or comma-separated) into a trimmed string array. */
function parseList(value: string): string[] {
	return value
		.split(/\r?\n|,/)
		.map((v) => v.trim())
		.filter((v) => v.length > 0);
}

/** Builds the create/update payload from the current form state. */
function buildPayload(formState: MCPFormState) {
	const base = { name: formState.name, description: formState.description };
	if (formState.type === "stdio") {
		return {
			...base,
			type: "stdio" as const,
			command: formState.command,
			args: parseList(formState.args),
			env: parseList(formState.env),
		};
	}
	return {
		...base,
		type: "http" as const,
		url: formState.url,
		headers: parseList(formState.headers),
	};
}

export function MCPServersDialog({ open, onOpenChange }: MCPServersDialogProps) {
	const {
		servers,
		isLoading,
		createMCPServer,
		updateMCPServer,
		deleteMCPServer,
	} = useMCPServers();

	const [selectedId, setSelectedId] = useState<string | null>(null);
	const [isNewMode, setIsNewMode] = useState(false);
	const [formState, setFormState] = useState<MCPFormState>(createEmptyFormState);
	const [isSubmitting, setIsSubmitting] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const selectedServer: MCPServer | undefined = useMemo(
		() => servers.find((s) => s.id === selectedId),
		[servers, selectedId],
	);

	const handleSelect = (server: MCPServer) => {
		setSelectedId(server.id);
		setIsNewMode(false);
		setFormState({
			name: server.name,
			description: server.description ?? "",
			type: server.type,
			command: server.command ?? "",
			args: (server.args ?? []).join("\n"),
			env: (server.env ?? []).join("\n"),
			url: server.url ?? "",
			headers: (server.headers ?? []).join("\n"),
		});
		setError(null);
	};

	const handleNew = () => {
		setSelectedId(null);
		setIsNewMode(true);
		setFormState(createEmptyFormState());
		setError(null);
	};

	const handleSave = async () => {
		if (!formState.name.trim()) {
			setError("Name is required");
			return;
		}
		if (formState.type === "stdio" && !formState.command.trim()) {
			setError("Command is required for stdio servers");
			return;
		}
		if (formState.type === "http" && !formState.url.trim()) {
			setError("URL is required for HTTP servers");
			return;
		}

		setIsSubmitting(true);
		setError(null);
		try {
			const payload = buildPayload(formState);
			if (selectedServer) {
				await updateMCPServer(selectedServer.id, payload);
			} else {
				const created = await createMCPServer(payload);
				setSelectedId(created.id);
				setIsNewMode(false);
			}
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to save server");
		} finally {
			setIsSubmitting(false);
		}
	};

	const handleDelete = async () => {
		if (!selectedServer) return;
		setIsSubmitting(true);
		setError(null);
		try {
			await deleteMCPServer(selectedServer.id);
			setSelectedId(null);
			setIsNewMode(false);
			setFormState(createEmptyFormState());
		} catch (err) {
			setError(err instanceof Error ? err.message : "Failed to delete server");
		} finally {
			setIsSubmitting(false);
		}
	};

	const handleClose = (nextOpen: boolean) => {
		if (!nextOpen) {
			setSelectedId(null);
			setIsNewMode(false);
			setFormState(createEmptyFormState());
			setError(null);
		}
		onOpenChange(nextOpen);
	};

	const setField = <K extends keyof MCPFormState>(key: K, value: MCPFormState[K]) =>
		setFormState((prev) => ({ ...prev, [key]: value }));

	return (
		<Dialog open={open} onOpenChange={handleClose}>
			<DialogContent className="sm:max-w-4xl max-h-[85vh] flex flex-col overflow-hidden">
				<DialogHeader>
					<DialogTitle>MCP Servers</DialogTitle>
					<DialogDescription>
						Manage MCP server configurations for this project.
					</DialogDescription>
				</DialogHeader>
				<div className="flex-1 flex flex-col gap-4 lg:flex-row min-w-0 min-h-0 overflow-hidden mt-4">
					{/* Left: server list */}
					<div className="w-full lg:w-64 flex flex-col border rounded-md p-3 bg-muted/30 min-h-0">
						<div className="flex items-center justify-between mb-2 shrink-0">
							<Label className="text-xs text-muted-foreground">Servers</Label>
							<button
								type="button"
								onClick={handleNew}
								className="h-6 w-6 flex items-center justify-center rounded-md hover:bg-muted text-muted-foreground hover:text-foreground transition-colors focus:outline-none"
								title="New server"
							>
								<Plus className="h-3.5 w-3.5" />
							</button>
						</div>
						<div className="flex-1 min-h-0 overflow-y-auto space-y-1 text-sm mt-2">
							{isLoading ? (
								<div className="py-4 text-center text-xs text-muted-foreground">
									Loading servers...
								</div>
							) : servers.length === 0 ? (
								<div className="py-4 text-center text-xs text-muted-foreground">
									No servers yet. Click <strong>New</strong> to get started.
								</div>
							) : (
								servers.map((server) => (
									<button
										key={server.id}
										type="button"
										onClick={() => handleSelect(server)}
										className={cn(
											"w-full text-left px-2 py-1.5 rounded-md border border-transparent hover:bg-muted/60 text-xs",
											selectedId === server.id &&
												"bg-background border-border shadow-xs",
										)}
									>
										<div className="flex items-center justify-between gap-2">
											<span className="truncate font-medium">{server.name}</span>
											<span className="text-[10px] text-muted-foreground flex items-center gap-1">
												{server.type === "stdio" ? (
													<><Terminal className="h-3 w-3" />Stdio</>
												) : (
													<><Globe2 className="h-3 w-3" />HTTP</>
												)}
											</span>
										</div>
										{server.description && (
											<p className="mt-0.5 line-clamp-2 text-[11px] text-muted-foreground">
												{server.description}
											</p>
										)}
									</button>
								))
							)}
						</div>
					</div>

					{/* Right: form or empty state */}
					{selectedId === null && !isNewMode ? (
						<div className="flex-1 flex flex-col items-center justify-center text-center text-muted-foreground gap-3">
							<Server className="h-8 w-8 opacity-25" />
							<p className="text-sm">
								Select a server from the list or click the{" "}
								<strong className="text-foreground">+</strong> button to create one.
							</p>
						</div>
					) : (
						<div className="flex-1 min-w-0 flex flex-col overflow-hidden">
							<div className="flex-1 overflow-y-auto space-y-4 pr-1">
								{/* Name */}
								<div className="space-y-1.5">
									<Label htmlFor="mcp-name">Name</Label>
									<Input
										id="mcp-name"
										value={formState.name}
										onChange={(e) => setField("name", e.target.value)}
										placeholder="e.g. Local Filesystem"
									/>
								</div>

								{/* Description */}
								<div className="space-y-1.5">
									<Label htmlFor="mcp-description">Description</Label>
									<Input
										id="mcp-description"
										value={formState.description}
										onChange={(e) => setField("description", e.target.value)}
										placeholder="e.g. Provides access to the local filesystem"
									/>
								</div>

								{/* Type selector */}
								<div className="space-y-2">
									<Label>Type</Label>
									<div className="inline-flex items-center gap-2 rounded-md border bg-muted/40 p-1 text-xs">
										<button
											type="button"
											onClick={() => setField("type", "stdio")}
											className={cn(
												"px-2 py-1 rounded-md",
												formState.type === "stdio" && "bg-background shadow-xs",
											)}
										>
											Stdio
										</button>
										<button
											type="button"
											onClick={() => setField("type", "http")}
											className={cn(
												"px-2 py-1 rounded-md",
												formState.type === "http" && "bg-background shadow-xs",
											)}
										>
											HTTP
										</button>
									</div>
								</div>

								{/* Type-specific fields */}
								{formState.type === "stdio" ? (
									<div className="space-y-4">
										<div className="space-y-1.5">
											<Label htmlFor="mcp-command">Command</Label>
											<Input
												id="mcp-command"
												value={formState.command}
												onChange={(e) => setField("command", e.target.value)}
												placeholder="e.g. node ./mcp-server.js"
											/>
										</div>
										<div className="grid grid-cols-2 gap-4">
											<div className="space-y-1.5">
												<Label htmlFor="mcp-args">Args (one per line)</Label>
												<Textarea
													id="mcp-args"
													value={formState.args}
													onChange={(e) => setField("args", e.target.value)}
													placeholder={"--flag\n--another=123"}
													className="min-h-[100px] text-xs font-mono"
												/>
											</div>
											<div className="space-y-1.5">
												<Label htmlFor="mcp-env">
													Env (KEY=value, one per line)
												</Label>
												<Textarea
													id="mcp-env"
													value={formState.env}
													onChange={(e) => setField("env", e.target.value)}
													placeholder="API_KEY=..."
													className="min-h-[100px] text-xs font-mono"
												/>
											</div>
										</div>
									</div>
								) : (
									<div className="space-y-4">
										<div className="space-y-1.5">
											<Label htmlFor="mcp-url">URL</Label>
											<Input
												id="mcp-url"
												value={formState.url}
												onChange={(e) => setField("url", e.target.value)}
												placeholder="https://example.com/mcp"
											/>
										</div>
										<div className="space-y-1.5">
											<Label htmlFor="mcp-headers">
												Headers (Name: Value, one per line)
											</Label>
											<Textarea
												id="mcp-headers"
												value={formState.headers}
												onChange={(e) => setField("headers", e.target.value)}
												placeholder="Authorization: Bearer ..."
												className="min-h-[100px] text-xs font-mono"
											/>
										</div>
									</div>
								)}
							</div>

							{/* Footer: error + Save / Delete */}
							<div className="flex items-center justify-between pt-3 border-t mt-3 shrink-0">
								{error ? (
									<p className="text-xs text-destructive">{error}</p>
								) : (
									<span />
								)}
								<div className="flex items-center gap-2">
									<Button
										variant="ghost"
										size="sm"
										disabled={isSubmitting || !selectedServer}
										onClick={handleDelete}
									>
										<Trash2 className="h-3.5 w-3.5 mr-1" />
										Delete
									</Button>
									<Button
										size="sm"
										disabled={isSubmitting || !formState.name}
										onClick={handleSave}
									>
										{isSubmitting ? "Saving..." : "Save"}
									</Button>
								</div>
							</div>
						</div>
					)}
				</div>
			</DialogContent>
		</Dialog>
	);
}
