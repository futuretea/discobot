import {
	AlertTriangle,
	CheckCircle,
	Clock,
	Loader2,
	RotateCcw,
	XCircle,
	Zap,
} from "lucide-react";
import * as React from "react";
import {
	QueueItem,
	QueueItemContent,
	QueueList,
	QueueSection,
	QueueSectionContent,
	QueueSectionLabel,
	QueueSectionTrigger,
} from "@/components/ai-elements/queue";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { api } from "@/lib/api-client";
import type { HookRunStatus, HooksStatusResponse } from "@/lib/api-types";
import { cn } from "@/lib/utils";

// Context for sharing state between button and panel
interface HookStatusContextValue {
	isExpanded: boolean;
	toggle: () => void;
	passedCount: number;
	totalCount: number;
	hasFailures: boolean;
	hasRunning: boolean;
	sessionId: string | null;
}

const HookStatusContext = React.createContext<HookStatusContextValue | null>(
	null,
);

function useHookStatusContext() {
	return React.useContext(HookStatusContext);
}

interface ChatHookStatusProps {
	hooksStatus: HooksStatusResponse | null;
	sessionId: string | null;
	children?: React.ReactNode;
}

/**
 * ChatHookStatus - Provider for hook status compound component.
 * Renders nothing if no hooks have run yet.
 */
export function ChatHookStatus({
	hooksStatus,
	sessionId,
	children,
}: ChatHookStatusProps) {
	const [isExpanded, setIsExpanded] = React.useState(false);

	const hooks = Object.values(hooksStatus?.hooks ?? {});
	const pendingSet = new Set(hooksStatus?.pendingHooks ?? []);
	const passedCount = hooks.filter(
		(h) => h.lastResult === "success" && !pendingSet.has(h.hookId),
	).length;
	const totalCount = hooks.length;
	const hasFailures = hooks.some((h) => h.lastResult === "failure");
	const hasRunning = hooks.some((h) => h.lastResult === "running");

	const toggle = React.useCallback(() => {
		setIsExpanded((prev) => !prev);
	}, []);

	const contextValue: HookStatusContextValue = React.useMemo(
		() => ({
			isExpanded,
			toggle,
			passedCount,
			totalCount,
			hasFailures,
			hasRunning,
			sessionId,
		}),
		[
			isExpanded,
			toggle,
			passedCount,
			totalCount,
			hasFailures,
			hasRunning,
			sessionId,
		],
	);

	// Don't provide context if no hooks have run
	if (totalCount === 0) {
		return <>{children}</>;
	}

	return (
		<HookStatusContext.Provider value={contextValue}>
			{children}
		</HookStatusContext.Provider>
	);
}

/**
 * HookStatusButton - Minimal button showing hook pass/fail count.
 * Renders in the input footer. Returns null when no hooks exist.
 */
export const HookStatusButton = React.memo(function HookStatusButton() {
	const context = useHookStatusContext();

	if (!context) {
		return null;
	}

	const { passedCount, hasFailures, hasRunning, toggle } = context;

	return (
		<Button
			variant="ghost"
			className="gap-1.5 h-8 px-2"
			onClick={toggle}
			type="button"
		>
			{hasRunning ? (
				<Loader2 className="h-3.5 w-3.5 animate-spin text-blue-500" />
			) : hasFailures ? (
				<AlertTriangle className="h-3.5 w-3.5 text-yellow-500" />
			) : (
				<Zap className="h-3.5 w-3.5 text-green-500" />
			)}
			<span className="text-xs font-medium">{passedCount}</span>
		</Button>
	);
});

interface HookStatusPanelProps {
	hooksStatus: HooksStatusResponse;
}

/**
 * HookStatusPanel - Expanded panel showing all hooks and their status.
 * Renders above the input when expanded. Returns null when no hooks exist.
 */
export const HookStatusPanel = React.memo(function HookStatusPanel({
	hooksStatus,
}: HookStatusPanelProps) {
	const context = useHookStatusContext();
	const [selectedHook, setSelectedHook] = React.useState<{
		hook: HookRunStatus;
		isPending: boolean;
	} | null>(null);

	if (!context) {
		return null;
	}

	const { isExpanded, passedCount, sessionId } = context;

	if (!isExpanded) {
		return null;
	}

	const hooks = Object.values(hooksStatus.hooks);
	const pendingSet = new Set(hooksStatus.pendingHooks);

	return (
		<>
			<div className="mb-2 rounded-lg border bg-background shadow-sm animate-in slide-in-from-bottom-2">
				<QueueSection defaultOpen={true}>
					<QueueSectionTrigger>
						<QueueSectionLabel
							count={hooks.length}
							label={`Hooks (${passedCount} passed)`}
						/>
					</QueueSectionTrigger>
					<QueueSectionContent>
						<QueueList>
							{hooks.map((hook) => (
								<HookStatusItem
									key={hook.hookId}
									hook={hook}
									isPending={pendingSet.has(hook.hookId)}
									onClick={() =>
										setSelectedHook({
											hook,
											isPending: pendingSet.has(hook.hookId),
										})
									}
								/>
							))}
						</QueueList>
					</QueueSectionContent>
				</QueueSection>
			</div>
			<HookDetailDialog
				hook={selectedHook?.hook ?? null}
				isPending={selectedHook?.isPending ?? false}
				sessionId={sessionId}
				onClose={() => setSelectedHook(null)}
			/>
		</>
	);
});

function HookStatusItem({
	hook,
	isPending,
	onClick,
}: {
	hook: HookRunStatus;
	isPending: boolean;
	onClick: () => void;
}) {
	const context = useHookStatusContext();
	const [rerunning, setRerunning] = React.useState(false);
	const isRunning = hook.lastResult === "running";
	const isSuccess = hook.lastResult === "success";
	const isFailure = hook.lastResult === "failure";
	const showRerun = (isFailure || isPending) && !isRunning && !rerunning;

	const handleRerun = React.useCallback(
		async (e: React.MouseEvent) => {
			e.stopPropagation();
			if (!context?.sessionId) return;
			setRerunning(true);
			try {
				await api.rerunHook(context.sessionId, hook.hookId);
			} catch (err) {
				console.error("Failed to rerun hook:", err);
			} finally {
				setRerunning(false);
			}
		},
		[context?.sessionId, hook.hookId],
	);

	return (
		<QueueItem
			className={cn(
				"cursor-pointer hover:bg-muted/80 transition-colors",
				isRunning && "bg-blue-500/10",
				isPending && !isRunning && "bg-muted/50",
				isFailure && !isPending && "bg-red-500/5",
			)}
			onClick={onClick}
		>
			<div className="flex items-center gap-2">
				{isRunning || rerunning ? (
					<Loader2 className="h-3 w-3 text-blue-500 animate-spin shrink-0" />
				) : isPending ? (
					<Clock className="h-3 w-3 text-muted-foreground shrink-0" />
				) : isSuccess ? (
					<CheckCircle className="h-3 w-3 text-green-500 shrink-0" />
				) : isFailure ? (
					<XCircle className="h-3 w-3 text-red-500 shrink-0" />
				) : (
					<span className="h-3 w-3 rounded-full border border-muted-foreground/50 shrink-0" />
				)}
				<QueueItemContent>{hook.hookName}</QueueItemContent>
				<span className="text-xs text-muted-foreground/60 shrink-0">
					{hook.type}
				</span>
				{showRerun && (
					<button
						type="button"
						className="ml-auto p-1 rounded hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
						onClick={handleRerun}
						title="Rerun hook"
					>
						<RotateCcw className="h-3 w-3" />
					</button>
				)}
			</div>
		</QueueItem>
	);
}

// --- Hook Detail Dialog ---

function HookDetailDialog({
	hook,
	isPending,
	sessionId,
	onClose,
}: {
	hook: HookRunStatus | null;
	isPending: boolean;
	sessionId: string | null;
	onClose: () => void;
}) {
	const [output, setOutput] = React.useState<string | null>(null);
	const [loadingOutput, setLoadingOutput] = React.useState(false);
	const [rerunning, setRerunning] = React.useState(false);

	// Fetch hook output when dialog opens
	React.useEffect(() => {
		if (!hook || !sessionId) {
			setOutput(null);
			return;
		}

		let cancelled = false;
		setLoadingOutput(true);
		setOutput(null);

		api
			.getHookOutput(sessionId, hook.hookId)
			.then((res) => {
				if (!cancelled) {
					setOutput(res.output);
				}
			})
			.catch(() => {
				if (!cancelled) {
					setOutput(null);
				}
			})
			.finally(() => {
				if (!cancelled) {
					setLoadingOutput(false);
				}
			});

		return () => {
			cancelled = true;
		};
	}, [hook, sessionId]);

	const handleRerun = React.useCallback(async () => {
		if (!hook || !sessionId) return;
		setRerunning(true);
		try {
			await api.rerunHook(sessionId, hook.hookId);
		} catch (err) {
			console.error("Failed to rerun hook:", err);
		} finally {
			setRerunning(false);
		}
	}, [hook, sessionId]);

	if (!hook) return null;

	const isRunning = hook.lastResult === "running";
	const isSuccess = hook.lastResult === "success";
	const isFailure = hook.lastResult === "failure";
	const canRerun = (isFailure || isPending) && !rerunning;

	return (
		<Dialog open={!!hook} onOpenChange={(open) => !open && onClose()}>
			<DialogContent className="sm:max-w-5xl max-h-[85vh] flex flex-col overflow-hidden">
				<DialogHeader>
					<DialogTitle className="flex items-center gap-2">
						<HookStatusIcon
							isPending={isPending}
							isRunning={isRunning}
							isSuccess={isSuccess}
							isFailure={isFailure}
						/>
						{hook.hookName}
					</DialogTitle>
					<DialogDescription>
						{hook.type} hook &middot;{" "}
						{hook.lastRunAt
							? `Last run ${formatRelativeTime(hook.lastRunAt)}`
							: "Never run"}
					</DialogDescription>
				</DialogHeader>

				{/* Stats row */}
				<div className="flex items-center gap-4 text-sm">
					<StatusBadge
						isPending={isPending}
						isRunning={isRunning}
						isSuccess={isSuccess}
						isFailure={isFailure}
					/>
					{!isRunning && (
						<span className="text-muted-foreground">
							Exit code: {hook.lastExitCode}
						</span>
					)}
					<span className="text-muted-foreground">Runs: {hook.runCount}</span>
					{hook.failCount > 0 && (
						<span className="text-red-500/80">Failures: {hook.failCount}</span>
					)}
					{canRerun && (
						<button
							type="button"
							className="ml-auto inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium border hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
							onClick={handleRerun}
						>
							<RotateCcw className="h-3 w-3" />
							Rerun
						</button>
					)}
					{rerunning && (
						<span className="ml-auto inline-flex items-center gap-1.5 text-xs text-muted-foreground">
							<Loader2 className="h-3 w-3 animate-spin" />
							Running...
						</span>
					)}
				</div>

				{/* Output log */}
				<div className="flex-1 min-h-0 overflow-hidden rounded-md border bg-muted/30">
					<div className="px-3 py-2 border-b bg-muted/50 text-xs font-medium text-muted-foreground">
						Output
					</div>
					<div className="overflow-auto max-h-[50vh]">
						{loadingOutput ? (
							<div className="flex items-center justify-center py-8 text-muted-foreground">
								<Loader2 className="h-4 w-4 animate-spin mr-2" />
								Loading output...
							</div>
						) : output ? (
							<OutputWithLineNumbers output={output} />
						) : (
							<div className="py-8 text-center text-sm text-muted-foreground">
								No output available
							</div>
						)}
					</div>
				</div>
			</DialogContent>
		</Dialog>
	);
}

function OutputWithLineNumbers({ output }: { output: string }) {
	const lines = output.split("\n");
	// Width of the line number gutter based on digit count
	const gutterWidth = `${String(lines.length).length + 1}ch`;

	return (
		<div className="flex text-xs font-mono leading-relaxed">
			<div
				className="shrink-0 select-none border-r bg-muted/50 text-muted-foreground/50 text-right py-3 pr-2 pl-2"
				style={{ minWidth: gutterWidth }}
				aria-hidden="true"
			>
				{lines.map((_, i) => (
					// biome-ignore lint/suspicious/noArrayIndexKey: line numbers are stable and index-based
					<div key={i}>{i + 1}</div>
				))}
			</div>
			<pre className="flex-1 p-3 whitespace-pre-wrap break-all overflow-x-auto">
				{output}
			</pre>
		</div>
	);
}

function HookStatusIcon({
	isPending,
	isRunning,
	isSuccess,
	isFailure,
}: {
	isPending?: boolean;
	isRunning: boolean;
	isSuccess: boolean;
	isFailure: boolean;
}) {
	if (isRunning)
		return <Loader2 className="h-5 w-5 text-blue-500 animate-spin shrink-0" />;
	if (isPending)
		return <Clock className="h-5 w-5 text-muted-foreground shrink-0" />;
	if (isSuccess)
		return <CheckCircle className="h-5 w-5 text-green-500 shrink-0" />;
	if (isFailure) return <XCircle className="h-5 w-5 text-red-500 shrink-0" />;
	return <Clock className="h-5 w-5 text-muted-foreground shrink-0" />;
}

function StatusBadge({
	isPending,
	isRunning,
	isSuccess,
	isFailure,
}: {
	isPending?: boolean;
	isRunning: boolean;
	isSuccess: boolean;
	isFailure: boolean;
}) {
	if (isRunning) {
		return (
			<span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium bg-blue-500/10 text-blue-500">
				<Loader2 className="h-3 w-3 animate-spin" />
				Running
			</span>
		);
	}
	if (isPending) {
		return (
			<span className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-muted text-muted-foreground">
				Pending
			</span>
		);
	}
	if (isSuccess) {
		return (
			<span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium bg-green-500/10 text-green-500">
				Passed
			</span>
		);
	}
	if (isFailure) {
		return (
			<span className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium bg-red-500/10 text-red-500">
				Failed
			</span>
		);
	}
	return (
		<span className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-muted text-muted-foreground">
			Pending
		</span>
	);
}

function formatRelativeTime(isoString: string): string {
	const date = new Date(isoString);
	const now = new Date();
	const diffMs = now.getTime() - date.getTime();
	const diffSec = Math.floor(diffMs / 1000);

	if (diffSec < 5) return "just now";
	if (diffSec < 60) return `${diffSec}s ago`;
	const diffMin = Math.floor(diffSec / 60);
	if (diffMin < 60) return `${diffMin}m ago`;
	const diffHr = Math.floor(diffMin / 60);
	if (diffHr < 24) return `${diffHr}h ago`;
	return date.toLocaleDateString();
}
