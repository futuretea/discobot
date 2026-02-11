import type { UIMessage } from "ai";
import * as React from "react";
import { api } from "@/lib/api-client";

interface UseMessagesOnceResult {
	messages: UIMessage[];
	isLoading: boolean;
	error: Error | null;
}

/**
 * Fetches messages once for a session (no caching).
 * Only uses the INITIAL sessionId value - does not re-fetch if sessionId changes.
 *
 * @param sessionId - The session ID to fetch messages for, or null to skip fetching
 * @returns Object containing messages array, loading state, and error state
 */
export function useMessagesOnce(
	sessionId: string | null,
): UseMessagesOnceResult {
	// Capture the initial sessionId value and never change it
	const initialSessionIdRef = React.useRef(sessionId);

	const [messages, setMessages] = React.useState<UIMessage[]>([]);
	const [isLoading, setIsLoading] = React.useState(false);
	const [error, setError] = React.useState<Error | null>(null);

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
			});

		return () => {
			cancelled = true;
		};
		// Empty dependency array - only runs once on mount
	}, []);

	return { messages, isLoading, error };
}
