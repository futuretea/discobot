import type { UIMessage } from "ai";
import * as React from "react";
import { api } from "@/lib/api-client";

interface UseMessagesOnceResult {
	messages: UIMessage[];
	isLoading: boolean;
	error: Error | null;
	retry: () => void;
}

/**
 * Fetches messages once for a session (no caching).
 * Only uses the INITIAL sessionId value - does not re-fetch if sessionId changes.
 * Automatically retries after 5 seconds on error. Also exposes a manual retry function.
 *
 * @param sessionId - The session ID to fetch messages for, or null to skip fetching
 * @returns Object containing messages array, loading state, error state, and retry function
 */
export function useMessagesOnce(
	sessionId: string | null,
): UseMessagesOnceResult {
	// Capture the initial sessionId value and never change it
	const initialSessionIdRef = React.useRef(sessionId);

	const [messages, setMessages] = React.useState<UIMessage[]>([]);
	const [isLoading, setIsLoading] = React.useState(false);
	const [error, setError] = React.useState<Error | null>(null);
	const [retryCount, setRetryCount] = React.useState(0);

	const retry = React.useCallback(() => {
		setRetryCount((c) => c + 1);
	}, []);

	// biome-ignore lint/correctness/useExhaustiveDependencies: retryCount is intentionally used as a trigger to re-run the fetch
	React.useEffect(() => {
		const initialSessionId = initialSessionIdRef.current;

		// Skip if no session ID provided initially
		if (!initialSessionId) {
			setMessages([]);
			setIsLoading(false);
			setError(null);
			return;
		}

		let cancelled = false;
		setIsLoading(true);
		setError(null);

		let autoRetryTimer: ReturnType<typeof setTimeout> | undefined;

		api
			.getMessages(initialSessionId)
			.then((data) => {
				if (cancelled) return;
				setMessages(data.messages || []);
				setIsLoading(false);
			})
			.catch((err) => {
				if (cancelled) return;
				setError(err);
				setIsLoading(false);
				autoRetryTimer = setTimeout(() => {
					if (!cancelled) setRetryCount((c) => c + 1);
				}, 5000);
			});

		return () => {
			cancelled = true;
			clearTimeout(autoRetryTimer);
		};
	}, [retryCount]);

	return { messages, isLoading, error, retry };
}
