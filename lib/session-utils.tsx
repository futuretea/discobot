import { AlertCircle, Check, Circle, Loader2, Pause } from "lucide-react";
import {
	CommitStatus,
	SessionStatus as SessionStatusConstants,
} from "@/lib/api-constants";
import type { Session } from "@/lib/api-types";

/**
 * Get hover text for a session, showing status or error message.
 */
export function getSessionHoverText(session: Session): string {
	// Show commit error if commit failed
	if (session.commitStatus === CommitStatus.FAILED && session.commitError) {
		return `Commit Failed: ${session.commitError}`;
	}

	const status = session.status
		.replace(/_/g, " ")
		.replace(/\b\w/g, (c) => c.toUpperCase());
	if (session.status === SessionStatusConstants.ERROR && session.errorMessage) {
		return `${status}: ${session.errorMessage}`;
	}
	return status;
}

/**
 * Get the status indicator icon for a session.
 * Shows commit status when relevant, otherwise session lifecycle status.
 * @param session - The session to get the status indicator for
 * @param size - The size variant: "default" (3-3.5) or "small" (2.5)
 */
export function getSessionStatusIndicator(
	session: Session,
	size: "default" | "small" = "default",
) {
	const iconSize = size === "small" ? "h-2.5 w-2.5" : "h-3.5 w-3.5";
	const smallIconSize = size === "small" ? "h-2.5 w-2.5" : "h-3 w-3";
	// Show commit status indicator if commit is in progress, failed, or completed
	if (
		session.commitStatus === CommitStatus.PENDING ||
		session.commitStatus === CommitStatus.COMMITTING
	) {
		return <Loader2 className={`${iconSize} text-blue-500 animate-spin`} />;
	}
	if (session.commitStatus === CommitStatus.FAILED) {
		return <AlertCircle className={`${iconSize} text-destructive`} />;
	}
	if (session.commitStatus === CommitStatus.COMPLETED) {
		return <Check className={`${iconSize} text-green-500`} />;
	}

	// Show session lifecycle status
	switch (session.status) {
		case SessionStatusConstants.INITIALIZING:
		case SessionStatusConstants.REINITIALIZING:
		case SessionStatusConstants.CLONING:
		case SessionStatusConstants.PULLING_IMAGE:
		case SessionStatusConstants.CREATING_SANDBOX:
			return <Loader2 className={`${iconSize} text-yellow-500 animate-spin`} />;
		case SessionStatusConstants.READY:
			return (
				<Circle className={`${smallIconSize} text-green-500 fill-green-500`} />
			);
		case SessionStatusConstants.RUNNING:
			return <Loader2 className={`${iconSize} text-blue-500 animate-spin`} />;
		case SessionStatusConstants.STOPPED:
			return <Pause className={`${iconSize} text-muted-foreground`} />;
		case SessionStatusConstants.ERROR:
			return size === "small" ? (
				<Circle
					className={`${smallIconSize} text-destructive fill-destructive`}
				/>
			) : (
				<AlertCircle className={`${iconSize} text-destructive`} />
			);
		case SessionStatusConstants.REMOVING:
			return <Loader2 className={`${iconSize} text-red-500 animate-spin`} />;
		default:
			return <Circle className={`${smallIconSize} text-muted-foreground`} />;
	}
}
