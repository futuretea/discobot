import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../api-client";
import type { SearchFileEntry } from "../api-types";

export type FileMentionItem = Pick<SearchFileEntry, "path" | "type">;

/**
 * Hook for fuzzy-searching workspace files to power @ mention autocomplete.
 * Calls the /files/search API which uses an fzf-style scoring algorithm
 * and returns both files and directories in ranked order.
 */
export function useFileMentionSearch(
	sessionId: string | null,
	enabled: boolean,
	query: string,
) {
	const [suggestions, setSuggestions] = useState<FileMentionItem[]>([]);
	const [isLoading, setIsLoading] = useState(false);

	// Debounce: only fire after user pauses briefly
	const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
	// Track the latest query to discard stale responses
	const latestQueryRef = useRef<string>("");

	const doSearch = useCallback(
		async (q: string) => {
			if (!sessionId) return;
			latestQueryRef.current = q;
			setIsLoading(true);
			try {
				const res = await api.searchSessionFiles(sessionId, q, 50);
				// Discard if a newer query has been issued
				if (latestQueryRef.current !== q) return;
				setSuggestions(
					res.results.map((r) => ({ path: r.path, type: r.type })),
				);
			} catch {
				if (latestQueryRef.current !== q) return;
				setSuggestions([]);
			} finally {
				if (latestQueryRef.current === q) {
					setIsLoading(false);
				}
			}
		},
		[sessionId],
	);

	useEffect(() => {
		if (!enabled || !sessionId) {
			setSuggestions([]);
			setIsLoading(false);
			return;
		}

		// Debounce: immediate for empty query (first open), small delay for typing
		const delay = query === "" ? 0 : 80;

		if (debounceRef.current) clearTimeout(debounceRef.current);
		debounceRef.current = setTimeout(() => {
			doSearch(query);
		}, delay);

		return () => {
			if (debounceRef.current) clearTimeout(debounceRef.current);
		};
	}, [enabled, sessionId, query, doSearch]);

	// Clear on disable
	useEffect(() => {
		if (!enabled) {
			setSuggestions([]);
			setIsLoading(false);
		}
	}, [enabled]);

	return { suggestions, isLoading };
}
