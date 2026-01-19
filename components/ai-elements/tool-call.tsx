"use client";

import {
	CheckCircle2,
	ChevronDown,
	ChevronRight,
	Code,
	Edit3,
	FileText,
	FolderOpen,
	Globe,
	Loader2,
	Search,
	Terminal,
	Wrench,
	XCircle,
} from "lucide-react";
import * as React from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { cn } from "@/lib/utils";

// Tool call part structure from AI SDK
export interface ToolCallPart {
	type: "dynamic-tool";
	toolCallId: string;
	toolName: string;
	title?: string;
	state:
		| "input-streaming"
		| "input-available"
		| "output-available"
		| "output-error";
	input?: Record<string, unknown>;
	output?: string;
	errorText?: string;
}

interface ToolCallProps {
	part: ToolCallPart;
	className?: string;
}

// Map tool names to icons
function getToolIcon(toolName: string) {
	const iconClass = "h-4 w-4";
	const name = toolName.toLowerCase();

	if (
		name.includes("bash") ||
		name.includes("terminal") ||
		name.includes("shell")
	) {
		return <Terminal className={iconClass} />;
	}
	if (name.includes("read") || name.includes("file")) {
		return <FileText className={iconClass} />;
	}
	if (
		name.includes("search") ||
		name.includes("grep") ||
		name.includes("glob")
	) {
		return <Search className={iconClass} />;
	}
	if (name.includes("edit") || name.includes("write")) {
		return <Edit3 className={iconClass} />;
	}
	if (name.includes("ls") || name.includes("list") || name.includes("folder")) {
		return <FolderOpen className={iconClass} />;
	}
	if (name.includes("web") || name.includes("fetch") || name.includes("http")) {
		return <Globe className={iconClass} />;
	}
	if (name.includes("code") || name.includes("lsp")) {
		return <Code className={iconClass} />;
	}
	return <Wrench className={iconClass} />;
}

// Get status indicator based on tool state
function getStatusIndicator(state: ToolCallPart["state"]) {
	switch (state) {
		case "input-streaming":
			return (
				<span className="flex items-center gap-1 text-xs text-blue-500">
					<Loader2 className="h-3 w-3 animate-spin" />
					Running
				</span>
			);
		case "input-available":
			return (
				<span className="flex items-center gap-1 text-xs text-yellow-500">
					<Loader2 className="h-3 w-3 animate-spin" />
					Waiting
				</span>
			);
		case "output-available":
			return (
				<span className="flex items-center gap-1 text-xs text-green-500">
					<CheckCircle2 className="h-3 w-3" />
					Complete
				</span>
			);
		case "output-error":
			return (
				<span className="flex items-center gap-1 text-xs text-red-500">
					<XCircle className="h-3 w-3" />
					Error
				</span>
			);
		default:
			return null;
	}
}

// Format input for display
function formatInput(input: Record<string, unknown> | undefined): string {
	if (!input) return "";
	try {
		return JSON.stringify(input, null, 2);
	} catch {
		return String(input);
	}
}

// Format output for display - truncate if too long
function formatOutput(output: string | undefined, maxLines = 20): string {
	if (!output) return "";
	const lines = output.split("\n");
	if (lines.length <= maxLines) return output;
	return `${lines.slice(0, maxLines).join("\n")}\n... (${lines.length - maxLines} more lines)`;
}

export function ToolCall({ part, className }: ToolCallProps) {
	const [isExpanded, setIsExpanded] = React.useState(false);
	const hasOutput =
		part.state === "output-available" || part.state === "output-error";

	// Extract display title - prefer title field, fall back to tool name
	const displayTitle = part.title || part.toolName;

	return (
		<div
			className={cn(
				"rounded-lg border bg-muted/30 overflow-hidden",
				part.state === "output-error" && "border-red-500/50",
				className,
			)}
		>
			{/* Header */}
			<button
				type="button"
				onClick={() => setIsExpanded(!isExpanded)}
				className={cn(
					"w-full flex items-center gap-2 px-3 py-2 text-left",
					"hover:bg-muted/50 transition-colors",
				)}
			>
				{isExpanded ? (
					<ChevronDown className="h-4 w-4 text-muted-foreground shrink-0" />
				) : (
					<ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />
				)}
				<span className="text-muted-foreground shrink-0">
					{getToolIcon(part.toolName)}
				</span>
				<span className="font-mono text-sm truncate flex-1">
					{displayTitle}
				</span>
				{getStatusIndicator(part.state)}
			</button>

			{/* Expanded content */}
			{isExpanded && (
				<div className="border-t border-border">
					{/* Input section */}
					{part.input && Object.keys(part.input).length > 0 && (
						<div className="px-3 py-2 border-b border-border">
							<div className="text-xs font-medium text-muted-foreground mb-1">
								Input
							</div>
							<pre className="text-xs font-mono bg-background/50 p-2 rounded overflow-x-auto max-h-[200px] overflow-y-auto">
								{formatInput(part.input)}
							</pre>
						</div>
					)}

					{/* Output section */}
					{hasOutput && (
						<div className="px-3 py-2">
							<div
								className={cn(
									"text-xs font-medium mb-1",
									part.state === "output-error"
										? "text-red-500"
										: "text-muted-foreground",
								)}
							>
								{part.state === "output-error" ? "Error" : "Output"}
							</div>
							{part.state === "output-error" && part.errorText ? (
								<div className="text-xs bg-background/50 p-2 rounded overflow-x-auto max-h-[300px] overflow-y-auto">
									<Markdown
										remarkPlugins={[remarkGfm]}
										components={{
											p: ({ children }) => (
												<p className="mb-2 last:mb-0">{children}</p>
											),
											code: ({ children, className }) => {
												const isInline = !className;
												return isInline ? (
													<code className="bg-muted px-1 py-0.5 rounded font-mono">
														{children}
													</code>
												) : (
													<code
														className={cn(
															"block bg-muted p-2 rounded font-mono overflow-x-auto",
															className,
														)}
													>
														{children}
													</code>
												);
											},
											pre: ({ children }) => (
												<pre className="bg-muted p-2 rounded overflow-x-auto my-2">
													{children}
												</pre>
											),
										}}
									>
										{part.errorText}
									</Markdown>
								</div>
							) : (
								<pre className="text-xs font-mono bg-background/50 p-2 rounded overflow-x-auto max-h-[300px] overflow-y-auto whitespace-pre-wrap">
									{formatOutput(part.output)}
								</pre>
							)}
						</div>
					)}
				</div>
			)}
		</div>
	);
}

// Component to render a list of tool calls
interface ToolCallListProps {
	parts: ToolCallPart[];
	className?: string;
}

export function ToolCallList({ parts, className }: ToolCallListProps) {
	if (parts.length === 0) return null;

	return (
		<div className={cn("space-y-2", className)}>
			{parts.map((part) => (
				<ToolCall key={part.toolCallId} part={part} />
			))}
		</div>
	);
}
