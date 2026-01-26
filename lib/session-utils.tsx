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
 */
export function getSessionStatusIndicator(session: Session) {
	// Show commit status indicator if commit is in progress, failed, or completed
	if (
		session.commitStatus === CommitStatus.PENDING ||
		session.commitStatus === CommitStatus.COMMITTING
	) {
		return <Loader2 className="h-3.5 w-3.5 text-blue-500 animate-spin" />;
	}
	if (session.commitStatus === CommitStatus.FAILED) {
		return <AlertCircle className="h-3.5 w-3.5 text-destructive" />;
	}
	if (session.commitStatus === CommitStatus.COMPLETED) {
		return <Check className="h-3.5 w-3.5 text-green-500" />;
	}

	// Show session lifecycle status
	switch (session.status) {
		case SessionStatusConstants.INITIALIZING:
		case SessionStatusConstants.REINITIALIZING:
		case SessionStatusConstants.CLONING:
		case SessionStatusConstants.PULLING_IMAGE:
		case SessionStatusConstants.CREATING_SANDBOX:
			return <Loader2 className="h-3.5 w-3.5 text-yellow-500 animate-spin" />;
		case SessionStatusConstants.READY:
			return <Circle className="h-3 w-3 text-green-500 fill-green-500" />;
		case SessionStatusConstants.STOPPED:
			return <Pause className="h-3.5 w-3.5 text-muted-foreground" />;
		case SessionStatusConstants.ERROR:
			return <AlertCircle className="h-3.5 w-3.5 text-destructive" />;
		case SessionStatusConstants.REMOVING:
			return <Loader2 className="h-3.5 w-3.5 text-red-500 animate-spin" />;
		default:
			return <Circle className="h-3 w-3 text-muted-foreground" />;
	}
}
