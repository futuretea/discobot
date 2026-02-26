import * as React from "react";

/**
 * Manages auto-retry with a live countdown when an error state is active.
 * Calls `onRetry` after `delayMs` ms and returns the remaining seconds for display.
 * Clears automatically when `isError` becomes false.
 */
export function useRetryCountdown(
	isError: boolean,
	onRetry: () => void,
	delayMs = 5000,
): number | null {
	const [countdown, setCountdown] = React.useState<number | null>(null);
	// Keep a stable ref so the timeout doesn't need onRetry in its deps
	const onRetryRef = React.useRef(onRetry);
	onRetryRef.current = onRetry;

	React.useEffect(() => {
		if (!isError) {
			setCountdown(null);
			return;
		}
		const seconds = Math.round(delayMs / 1000);
		setCountdown(seconds);
		const interval = setInterval(() => {
			setCountdown((prev) => (prev !== null && prev > 1 ? prev - 1 : null));
		}, 1000);
		const timer = setTimeout(() => {
			onRetryRef.current();
		}, delayMs);
		return () => {
			clearInterval(interval);
			clearTimeout(timer);
		};
	}, [isError, delayMs]);

	return countdown;
}
