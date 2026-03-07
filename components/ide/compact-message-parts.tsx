import type { UIMessage } from "ai";
import { ChevronDownIcon, ListIcon } from "lucide-react";
import React, { useMemo, useState } from "react";
import {
	Collapsible,
	CollapsibleContent,
	CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
	formatPartsSummary,
	groupPartsByType,
	groupToolsByName,
} from "./compact-message-parts-utils";
import { MessagePart } from "./message-parts";

interface CompactMessagePartsProps {
	message: UIMessage;
	isStreaming: boolean;
	role?: string;
}

const STREAMING_VISIBLE_PARTS = 2;

/**
 * Returns true when a part has reached a terminal state and will never change.
 * Used to set frozen=true so MessagePart can skip all re-renders for it.
 *
 * dynamic-tool terminal states: output-available | output-error | output-denied
 * Non-terminal states that must NOT be frozen: input-streaming, input-available,
 *   approval-requested, approval-responded (parallel tools may sit here while
 *   other parts continue updating).
 */
function isPartComplete(part: UIMessage["parts"][number]): boolean {
	if (part.type === "dynamic-tool") {
		return (
			part.state === "output-available" ||
			part.state === "output-error" ||
			part.state === "output-denied"
		);
	}
	if (part.type === "text" || part.type === "reasoning") {
		return part.state === "done";
	}
	// file, source-url, source-document, step-start — static, never change
	return true;
}

/**
 * CompactMessageParts renders message parts with automatic compaction.
 *
 * For assistant messages with 2+ parts:
 * - All parts except the last one are collapsed into a summary
 * - The summary shows counts by type (e.g., "2 Reads, 3 Writes, 1 text block, 1 reasoning block")
 * - Clicking the summary expands to show all parts
 * - Only the last part remains visible (typically the final result)
 *
 * While streaming, earlier parts collapse into a summary leaving only the most
 * recent two steps expanded.
 */
export const CompactMessageParts = React.memo(function CompactMessageParts({
	message,
	isStreaming,
	role,
}: CompactMessagePartsProps) {
	const totalParts = message.parts.length;
	const effectiveRole = role ?? message.role;

	// Don't collapse if any part is awaiting approval (user needs to interact)
	const hasActiveApproval = message.parts.some(
		(part) =>
			part.type === "dynamic-tool" && part.state === "approval-requested",
	);

	// During streaming, keep the view compact unless the user needs to approve a tool.
	const shouldUseStreamingCompaction =
		isStreaming &&
		!hasActiveApproval &&
		totalParts > STREAMING_VISIBLE_PARTS &&
		effectiveRole !== "user";

	// After streaming finishes, collapse earlier parts to a summary for assistants.
	const shouldUseCompactView =
		!isStreaming &&
		!hasActiveApproval &&
		totalParts >= 2 &&
		effectiveRole !== "user";

	if (shouldUseStreamingCompaction) {
		const visibleCount = Math.min(STREAMING_VISIBLE_PARTS, totalParts);
		const partsBeforeVisible = message.parts.slice(
			0,
			totalParts - visibleCount,
		);
		const visibleParts = message.parts.slice(totalParts - visibleCount);

		return (
			<>
				{partsBeforeVisible.length > 0 && (
					<PartsSummary
						message={message}
						parts={partsBeforeVisible}
						freezeParts={false}
					/>
				)}
				{visibleParts.map((part, idx) => {
					const partIdx = totalParts - visibleCount + idx;
					return (
						<MessagePart
							key={`${message.id}-part-${partIdx}`}
							message={message}
							partIdx={partIdx}
							part={part}
							frozen={isPartComplete(part)}
						/>
					);
				})}
			</>
		);
	}

	// If not using compact view, render all parts normally.
	// Freeze each part individually based on its own terminal state — position is
	// not reliable because parallel tool calls can leave multiple non-last parts
	// still in-flight (input-streaming, input-available, approval-requested, etc.).
	if (!shouldUseCompactView) {
		return (
			<>
				{message.parts.map((part, partIdx) => (
					<MessagePart
						key={`${message.id}-part-${partIdx}`}
						message={message}
						partIdx={partIdx}
						part={part}
						frozen={isPartComplete(part)}
					/>
				))}
			</>
		);
	}

	// Split into parts before last and the last part
	const partsBeforeLast = message.parts.slice(0, -1);
	const lastPart = message.parts[message.parts.length - 1];

	return (
		<>
			{/* Render collapsible summary for all parts except the last */}
			{partsBeforeLast.length > 0 && (
				<PartsSummary message={message} parts={partsBeforeLast} />
			)}

			{/* Render the last part (always visible) — compact view only shows after streaming, so frozen */}
			{lastPart && (
				<MessagePart
					key={`${message.id}-part-${totalParts - 1}`}
					message={message}
					partIdx={totalParts - 1}
					part={lastPart}
					frozen
				/>
			)}
		</>
	);
});

interface PartsSummaryProps {
	message: UIMessage;
	parts: UIMessage["parts"];
	freezeParts?: boolean;
}

/**
 * PartsSummary renders a collapsible summary of message parts.
 *
 * Default state: collapsed
 * Displays: "X parts • 2 Reads • 1 Write • 1 text block • 1 reasoning block"
 * When expanded: shows all individual parts using MessagePart
 */
const PartsSummary = React.memo(function PartsSummary({
	message,
	parts,
	freezeParts = true,
}: PartsSummaryProps) {
	const [isOpen, setIsOpen] = useState(false);

	// Compute part type counts and tool counts
	const { partCounts, toolCounts, totalParts } = useMemo(() => {
		const partCounts = groupPartsByType(parts);
		const toolCounts = groupToolsByName(parts);
		const totalParts = parts.length;

		return { partCounts, toolCounts, totalParts };
	}, [parts]);

	const summaryText = useMemo(() => {
		return formatPartsSummary(partCounts, toolCounts);
	}, [partCounts, toolCounts]);

	return (
		<Collapsible
			defaultOpen={false}
			open={isOpen}
			onOpenChange={setIsOpen}
			className="group not-prose mb-4 w-full rounded-md border border-transparent hover:border-border data-[state=open]:border-border transition-all duration-200"
		>
			<CollapsibleTrigger className="flex w-full items-center justify-between gap-4 p-3 hover:bg-muted/30 transition-all duration-200">
				<div className="flex items-center gap-2 opacity-0 group-hover:opacity-100 group-data-[state=open]:opacity-100 transition-all duration-200">
					<ListIcon className="size-4 text-muted-foreground" />
					<span className="font-medium text-sm">
						{totalParts} part{totalParts !== 1 ? "s" : ""}
					</span>
				</div>
				<div className="flex items-center gap-2">
					<span className="text-muted-foreground text-xs">{summaryText}</span>
					<ChevronDownIcon className="size-4 text-muted-foreground opacity-0 group-hover:opacity-100 group-data-[state=open]:opacity-100 transition-all duration-200 group-data-[state=open]:rotate-180" />
				</div>
			</CollapsibleTrigger>

			<CollapsibleContent className="border-t bg-muted/20 px-3 py-2">
				<div className="space-y-2">
					{parts.map((part, idx) => (
						<MessagePart
							key={`${message.id}-part-${idx}`}
							message={message}
							partIdx={idx}
							part={part}
							frozen={freezeParts ? true : isPartComplete(part)}
						/>
					))}
				</div>
			</CollapsibleContent>
		</Collapsible>
	);
});
