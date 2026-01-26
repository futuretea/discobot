"use client";

import { ChevronUpIcon, HistoryIcon } from "lucide-react";
import { memo, type RefObject, useEffect, useRef } from "react";
import { MAX_VISIBLE_HISTORY } from "@/lib/hooks/use-prompt-history";
import { cn } from "@/lib/utils";

export interface PromptHistoryDropdownProps {
	/** Full history array */
	history: string[];
	/** Currently selected index (-1 = none) */
	historyIndex: number;
	/** Whether dropdown is open */
	isHistoryOpen: boolean;
	/** Set the history index (for hover) */
	setHistoryIndex: (index: number) => void;
	/** Select a history item */
	onSelectHistory: (prompt: string) => void;
	/** Ref to the textarea (for click-outside detection) */
	textareaRef: RefObject<HTMLTextAreaElement | null>;
	/** Close the dropdown */
	closeHistory: () => void;
}

export const PromptHistoryDropdown = memo(function PromptHistoryDropdown({
	history: fullHistory,
	historyIndex,
	isHistoryOpen,
	setHistoryIndex,
	onSelectHistory,
	textareaRef,
	closeHistory,
}: PromptHistoryDropdownProps) {
	const dropdownRef = useRef<HTMLDivElement>(null);

	// Only show the most recent prompts
	const history = fullHistory.slice(0, MAX_VISIBLE_HISTORY);

	// Scroll selected item into view
	useEffect(() => {
		if (isHistoryOpen && historyIndex >= 0 && dropdownRef.current) {
			const selectedItem = dropdownRef.current.querySelector(
				`[data-index="${historyIndex}"]`,
			);
			if (selectedItem) {
				selectedItem.scrollIntoView({ block: "nearest" });
			}
		}
	}, [isHistoryOpen, historyIndex]);

	// Close dropdown when clicking outside
	useEffect(() => {
		if (!isHistoryOpen) return;

		const handleClickOutside = (e: MouseEvent) => {
			if (
				dropdownRef.current &&
				!dropdownRef.current.contains(e.target as Node) &&
				textareaRef.current &&
				!textareaRef.current.contains(e.target as Node)
			) {
				closeHistory();
			}
		};

		document.addEventListener("mousedown", handleClickOutside);
		return () => document.removeEventListener("mousedown", handleClickOutside);
	}, [isHistoryOpen, textareaRef, closeHistory]);

	if (!isHistoryOpen || history.length === 0) {
		return null;
	}

	return (
		<div
			ref={dropdownRef}
			className="absolute bottom-full left-0 right-0 z-50 mb-1 flex max-h-64 flex-col overflow-hidden rounded-lg border border-border bg-popover shadow-lg"
		>
			<div className="sticky top-0 z-10 flex items-center gap-2 border-b border-border bg-popover px-3 py-2">
				<HistoryIcon className="h-4 w-4 text-muted-foreground" />
				<span className="text-xs font-medium text-muted-foreground">
					Recent prompts
				</span>
				<span className="ml-auto text-xs text-muted-foreground">
					<ChevronUpIcon className="inline h-3 w-3" />
					<span className="mx-0.5">/</span>
					<ChevronUpIcon className="inline h-3 w-3 rotate-180" />
					to navigate
				</span>
			</div>
			<div className="flex flex-col-reverse overflow-y-auto py-1">
				{history.map((prompt, index) => (
					<button
						key={prompt}
						type="button"
						data-index={index}
						onClick={() => onSelectHistory(prompt)}
						onMouseEnter={() => setHistoryIndex(index)}
						className={cn(
							"w-full px-3 py-2 text-left text-sm transition-colors",
							"hover:bg-accent",
							index === historyIndex && "bg-accent",
						)}
					>
						<span className="line-clamp-2 break-words">{prompt}</span>
					</button>
				))}
			</div>
		</div>
	);
});
