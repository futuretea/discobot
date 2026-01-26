"use client";

import {
	type KeyboardEvent,
	type RefObject,
	useCallback,
	useEffect,
	useRef,
	useState,
} from "react";

// ============================================================================
// localStorage Helpers
// ============================================================================

const HISTORY_KEY = "octobot:prompt-history";
const DRAFT_PREFIX = "octobot-prompt-draft-";
const MAX_HISTORY_SIZE = 100;
export const MAX_VISIBLE_HISTORY = 20;

function loadHistory(): string[] {
	if (typeof window === "undefined") return [];
	try {
		const stored = localStorage.getItem(HISTORY_KEY);
		if (stored) {
			const parsed = JSON.parse(stored);
			if (Array.isArray(parsed)) {
				return parsed.filter((item) => typeof item === "string");
			}
		}
	} catch {
		// Ignore parse errors
	}
	return [];
}

function saveHistoryToStorage(history: string[]): void {
	if (typeof window === "undefined") return;
	try {
		const trimmed = history.slice(0, MAX_HISTORY_SIZE);
		localStorage.setItem(HISTORY_KEY, JSON.stringify(trimmed));
	} catch {
		// Ignore storage errors
	}
}

function getDraft(sessionId: string | null | undefined): string {
	if (typeof window === "undefined" || !sessionId) return "";
	try {
		return localStorage.getItem(`${DRAFT_PREFIX}${sessionId}`) || "";
	} catch {
		return "";
	}
}

function saveDraft(sessionId: string | null | undefined, value: string): void {
	if (typeof window === "undefined" || !sessionId) return;
	try {
		if (value) {
			localStorage.setItem(`${DRAFT_PREFIX}${sessionId}`, value);
		} else {
			localStorage.removeItem(`${DRAFT_PREFIX}${sessionId}`);
		}
	} catch {
		// Ignore storage errors
	}
}

// ============================================================================
// Hook
// ============================================================================

export interface UsePromptHistoryOptions {
	/** Ref to the textarea element */
	textareaRef: RefObject<HTMLTextAreaElement | null>;
	/** Session ID for draft persistence */
	sessionId?: string | null;
}

export interface UsePromptHistoryReturn {
	/** Array of history prompts (most recent first) */
	history: string[];
	/** Currently selected history index (-1 = none) */
	historyIndex: number;
	/** Whether history dropdown is open */
	isHistoryOpen: boolean;
	/** Set the history index */
	setHistoryIndex: (index: number) => void;
	/** Select a history item (sets textarea value and closes dropdown) */
	onSelectHistory: (prompt: string) => void;
	/** Add a prompt to history (call after successful submit) */
	addToHistory: (prompt: string) => void;
	/** Close the history dropdown */
	closeHistory: () => void;
	/** Keyboard handler to attach to textarea's onKeyDown */
	handleKeyDown: (e: KeyboardEvent<HTMLTextAreaElement>) => void;
	/** Get current textarea value */
	getValue: () => string;
}

export function usePromptHistory({
	textareaRef,
	sessionId,
}: UsePromptHistoryOptions): UsePromptHistoryReturn {
	// History state
	const [history, setHistory] = useState<string[]>(() => loadHistory());
	const [historyIndex, setHistoryIndex] = useState(-1);
	const [isHistoryOpen, setIsHistoryOpen] = useState(false);

	// Draft persistence refs (avoid re-renders on typing)
	const draftTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
	const prevSessionRef = useRef(sessionId);

	// Load draft when sessionId changes
	useEffect(() => {
		if (prevSessionRef.current !== sessionId) {
			prevSessionRef.current = sessionId;
			const draft = getDraft(sessionId);
			if (textareaRef.current) {
				textareaRef.current.value = draft;
			}
		}
	}, [sessionId, textareaRef]);

	// Save draft on input (debounced) - attach to textarea
	useEffect(() => {
		const textarea = textareaRef.current;
		if (!textarea || !sessionId) return;

		const handleInput = () => {
			if (draftTimerRef.current) {
				clearTimeout(draftTimerRef.current);
			}
			draftTimerRef.current = setTimeout(() => {
				saveDraft(sessionId, textarea.value);
			}, 300);
		};

		textarea.addEventListener("input", handleInput);
		return () => {
			textarea.removeEventListener("input", handleInput);
			if (draftTimerRef.current) {
				clearTimeout(draftTimerRef.current);
			}
		};
	}, [sessionId, textareaRef]);

	// Load initial draft on mount
	useEffect(() => {
		if (textareaRef.current && sessionId) {
			const draft = getDraft(sessionId);
			if (draft) {
				textareaRef.current.value = draft;
			}
		}
	}, [sessionId, textareaRef]);

	// Get current value
	const getValue = useCallback(() => {
		return textareaRef.current?.value ?? "";
	}, [textareaRef]);

	// Close history
	const closeHistory = useCallback(() => {
		setIsHistoryOpen(false);
		setHistoryIndex(-1);
	}, []);

	// Select history item
	const onSelectHistory = useCallback(
		(prompt: string) => {
			if (textareaRef.current) {
				textareaRef.current.value = prompt;
				textareaRef.current.focus();
			}
			closeHistory();
		},
		[textareaRef, closeHistory],
	);

	// Add to history and clear draft
	const addToHistory = useCallback(
		(prompt: string) => {
			if (!prompt.trim()) return;
			setHistory((prev) => {
				// Don't add duplicates
				if (prev.includes(prompt)) return prev;
				const updated = [prompt, ...prev].slice(0, MAX_HISTORY_SIZE);
				saveHistoryToStorage(updated);
				return updated;
			});
			// Also clear draft after successful submit
			if (sessionId) {
				saveDraft(sessionId, "");
			}
		},
		[sessionId],
	);

	// Keyboard handler for history navigation
	const handleKeyDown = useCallback(
		(e: KeyboardEvent<HTMLTextAreaElement>) => {
			const visibleHistoryLength = Math.min(
				history.length,
				MAX_VISIBLE_HISTORY,
			);

			// Handle Enter to select from history
			if (
				e.key === "Enter" &&
				!e.shiftKey &&
				isHistoryOpen &&
				historyIndex >= 0
			) {
				e.preventDefault();
				const selectedPrompt = history[historyIndex];
				if (selectedPrompt) {
					onSelectHistory(selectedPrompt);
				}
				return;
			}

			// Handle Escape to close history dropdown
			if (e.key === "Escape" && isHistoryOpen) {
				e.preventDefault();
				closeHistory();
				return;
			}

			// Handle Up arrow for history navigation
			if (e.key === "ArrowUp" && visibleHistoryLength > 0) {
				const cursorPosition = textareaRef.current?.selectionStart ?? 0;

				// Only trigger history if cursor is at the start (position 0)
				if (cursorPosition === 0) {
					e.preventDefault();

					if (!isHistoryOpen) {
						// Open history dropdown and select most recent item
						setIsHistoryOpen(true);
						setHistoryIndex(0);
					} else {
						// Navigate toward older items (higher index)
						if (historyIndex < visibleHistoryLength - 1) {
							setHistoryIndex(historyIndex + 1);
						}
					}
					return;
				}
			}

			// Handle Down arrow for history navigation
			if (e.key === "ArrowDown" && isHistoryOpen && visibleHistoryLength > 0) {
				e.preventDefault();
				// Navigate toward newer items (lower index)
				if (historyIndex <= 0) {
					closeHistory();
				} else {
					setHistoryIndex(historyIndex - 1);
				}
				return;
			}
		},
		[
			history,
			historyIndex,
			isHistoryOpen,
			onSelectHistory,
			closeHistory,
			textareaRef,
		],
	);

	return {
		history,
		historyIndex,
		isHistoryOpen,
		setHistoryIndex,
		onSelectHistory,
		addToHistory,
		closeHistory,
		handleKeyDown,
		getValue,
	};
}
