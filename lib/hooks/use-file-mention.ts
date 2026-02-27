import * as React from "react";
import { useFileMentionSearch } from "@/lib/hooks/use-file-mention-search";

interface UseFileMentionOptions {
	textareaRef: React.RefObject<HTMLTextAreaElement | null>;
	sessionId: string | null;
	isNewSession: boolean;
	/** True when the session sandbox is ready to serve file search requests */
	isSessionReady?: boolean;
	historyKeyDown: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
	onSelectHistory: (prompt: string) => void;
	/** Called when @ is typed in a new session to trigger session creation */
	onTriggerSessionCreate?: () => Promise<void>;
}

/**
 * Manages @ file mention autocomplete state and keyboard handling.
 * Wraps historyKeyDown so the textarea only needs one onKeyDown handler.
 *
 * For new sessions: if onTriggerSessionCreate is provided, typing @ will
 * create the session (showing a loading dropdown) then search files once ready.
 */
export function useFileMention({
	textareaRef,
	sessionId,
	isNewSession,
	isSessionReady = false,
	historyKeyDown,
	onSelectHistory,
	onTriggerSessionCreate,
}: UseFileMentionOptions) {
	const [isOpen, setIsOpen] = React.useState(false);
	const [query, setQuery] = React.useState("");
	const [triggerIndex, setTriggerIndex] = React.useState(0);
	const [selectedIndex, setSelectedIndex] = React.useState(0);
	const [isCreatingSession, setIsCreatingSession] = React.useState(false);
	// Stays true from the moment @ triggers creation until the first file
	// search completes — covers both the sandbox spin-up and the initial search.
	const [isInitializingSession, setIsInitializingSession] =
		React.useState(false);

	// Clear creation state when session finishes being created
	React.useEffect(() => {
		if (!isNewSession) {
			setIsCreatingSession(false);
		}
	}, [isNewSession]);

	const { suggestions, isLoading: isMentionLoading } = useFileMentionSearch(
		sessionId,
		// Only search once the sandbox is ready to serve file search requests
		isOpen && isSessionReady,
		query,
	);

	// Once the sandbox is ready and the first file search finishes,
	// clear the initializing flag.
	React.useEffect(() => {
		if (isInitializingSession && isSessionReady && !isMentionLoading) {
			setIsInitializingSession(false);
		}
	}, [isInitializingSession, isSessionReady, isMentionLoading]);

	const handleTextareaChange = React.useCallback(
		(e: React.ChangeEvent<HTMLTextAreaElement>) => {
			// For new sessions without a creation callback, skip mention handling
			if (isNewSession && !onTriggerSessionCreate) return;
			const value = e.currentTarget.value;
			const cursor = e.currentTarget.selectionStart ?? 0;
			const beforeCursor = value.slice(0, cursor);
			const match = beforeCursor.match(/@([^\s@]*)$/);
			if (match) {
				setQuery(match[1]);
				setTriggerIndex(cursor - match[0].length);
				setIsOpen(true);
				setSelectedIndex(0);
				// For new sessions, trigger session creation when @ is detected
				if (isNewSession && onTriggerSessionCreate && !isCreatingSession) {
					setIsCreatingSession(true);
					setIsInitializingSession(true);
					onTriggerSessionCreate().catch(() => {
						setIsCreatingSession(false);
						setIsInitializingSession(false);
						setIsOpen(false);
					});
				}
			} else {
				setIsOpen(false);
			}
		},
		[isNewSession, onTriggerSessionCreate, isCreatingSession],
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
			if (isOpen) {
				if (suggestions.length > 0) {
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
					}
				}
				if (e.key === "Escape") {
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
		isLoading: isInitializingSession || isMentionLoading,
		isInitializingSession,
		selectedIndex,
		handleTextareaChange,
		handleSelect,
		handleKeyDown,
		wrappedOnSelectHistory,
		dismiss: () => setIsOpen(false),
	};
}
