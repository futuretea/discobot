import * as React from "react";
import { useFileMentionSearch } from "@/lib/hooks/use-file-mention-search";

interface UseFileMentionOptions {
	textareaRef: React.RefObject<HTMLTextAreaElement | null>;
	sessionId: string | null;
	isNewSession: boolean;
	historyKeyDown: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
	onSelectHistory: (prompt: string) => void;
}

/**
 * Manages @ file mention autocomplete state and keyboard handling.
 * Wraps historyKeyDown so the textarea only needs one onKeyDown handler.
 */
export function useFileMention({
	textareaRef,
	sessionId,
	isNewSession,
	historyKeyDown,
	onSelectHistory,
}: UseFileMentionOptions) {
	const [isOpen, setIsOpen] = React.useState(false);
	const [query, setQuery] = React.useState("");
	const [triggerIndex, setTriggerIndex] = React.useState(0);
	const [selectedIndex, setSelectedIndex] = React.useState(0);

	const { suggestions, isLoading } = useFileMentionSearch(
		sessionId,
		isOpen,
		query,
	);

	const handleTextareaChange = React.useCallback(
		(e: React.ChangeEvent<HTMLTextAreaElement>) => {
			if (isNewSession) return;
			const value = e.currentTarget.value;
			const cursor = e.currentTarget.selectionStart ?? 0;
			const beforeCursor = value.slice(0, cursor);
			const match = beforeCursor.match(/@([^\s@]*)$/);
			if (match) {
				setQuery(match[1]);
				setTriggerIndex(cursor - match[0].length);
				setIsOpen(true);
				setSelectedIndex(0);
			} else {
				setIsOpen(false);
			}
		},
		[isNewSession],
	);

	const handleSelect = React.useCallback(
		(path: string) => {
			const textarea = textareaRef.current;
			if (!textarea) return;
			textarea.setRangeText(
				`@${path} `,
				triggerIndex,
				textarea.selectionStart ?? 0,
				"end",
			);
			setIsOpen(false);
			textarea.focus();
		},
		[textareaRef, triggerIndex],
	);

	const handleKeyDown = React.useCallback(
		(e: React.KeyboardEvent<HTMLTextAreaElement>) => {
			if (isOpen && suggestions.length > 0) {
				switch (e.key) {
					case "ArrowDown":
						e.preventDefault();
						setSelectedIndex((i) => Math.min(i + 1, suggestions.length - 1));
						return;
					case "ArrowUp":
						e.preventDefault();
						setSelectedIndex((i) => Math.max(i - 1, 0));
						return;
					case "Enter":
					case "Tab":
						e.preventDefault();
						if (suggestions[selectedIndex]) {
							handleSelect(suggestions[selectedIndex].path);
						}
						return;
					case "Escape":
						e.preventDefault();
						setIsOpen(false);
						return;
				}
			}
			historyKeyDown(e);
		},
		[isOpen, suggestions, selectedIndex, handleSelect, historyKeyDown],
	);

	// Close mention dropdown when a history entry is selected
	const wrappedOnSelectHistory = React.useCallback(
		(prompt: string) => {
			setIsOpen(false);
			onSelectHistory(prompt);
		},
		[onSelectHistory],
	);

	return {
		isOpen,
		query,
		suggestions,
		isLoading,
		selectedIndex,
		handleTextareaChange,
		handleSelect,
		handleKeyDown,
		wrappedOnSelectHistory,
		dismiss: () => setIsOpen(false),
	};
}
