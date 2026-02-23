import { exec } from "node:child_process";
import { promisify } from "node:util";
import type { UIMessage } from "ai";
import type { Agent } from "../agent/interface.js";
import { createUIMessage, generateMessageId } from "../agent/utils.js";
import type {
	CancelCompletionResponse,
	ChatConflictResponse,
	ChatRequest,
	ChatStartedResponse,
	ErrorResponse,
	NoActiveCompletionResponse,
} from "../api/types.js";
import {
	type CredentialEnvVar,
	checkCredentialsChanged,
} from "../credentials/credentials.js";
import type { HookManager } from "../hooks/manager.js";
import {
	addCompletionEvent,
	clearCompletionEvents,
	finishCompletion,
	getCompletionState,
	isCompletionRunning,
	startCompletion,
} from "../store/session.js";

const execAsync = promisify(exec);

/** Maximum consecutive hook-triggered re-prompts per user completion */
const MAX_HOOK_RETRIES = 3;

// Global state for the current completion
let currentAgent: Agent | null = null;
let currentSessionId: string | undefined;
let currentAbortController: AbortController | null = null;

// Hook evaluation state (non-blocking, runs after completion finishes)
let hookAbortController: AbortController | null = null;
let hookRetryCount = 0;

export type StartCompletionResult =
	| { ok: true; status: 202; response: ChatStartedResponse }
	| { ok: false; status: 409; response: ChatConflictResponse }
	| { ok: false; status: 400; response: ErrorResponse };

/**
 * Attempt to start a chat completion. Validates the request, checks for
 * conflicts, and starts the completion in the background if successful.
 *
 * Returns a result object with the appropriate response and status code.
 */
/**
 * Reset hook evaluation state. Call before user-initiated completions
 * to abort any running background hook evaluation and reset the retry counter.
 */
export function resetHookState(): void {
	if (hookAbortController) {
		hookAbortController.abort();
		hookAbortController = null;
	}
	hookRetryCount = 0;
}

export function tryStartCompletion(
	agent: Agent,
	body: ChatRequest,
	credentialsHeader: string | null,
	gitUserName: string | null,
	gitUserEmail: string | null,
	model: string | undefined,
	reasoning: "enabled" | "disabled" | "" | undefined,
	mode: "plan" | "" | undefined,
	sessionId?: string,
	hookManager?: HookManager | null,
): StartCompletionResult {
	const completionId = crypto.randomUUID().slice(0, 8);
	const log = (data: Record<string, unknown>) =>
		console.log(JSON.stringify({ completionId, ...data }));

	// Check if a completion is already running
	if (isCompletionRunning()) {
		const state = getCompletionState();
		log({ event: "conflict", existingCompletionId: state.completionId });
		return {
			ok: false,
			status: 409,
			response: {
				error: "completion_in_progress",
				completionId: state.completionId || "unknown",
			},
		};
	}

	const { messages: inputMessages } = body;

	if (!inputMessages || !Array.isArray(inputMessages)) {
		return {
			ok: false,
			status: 400,
			response: { error: "messages array required" },
		};
	}

	// Get the last user message to send
	const lastUserMessage = inputMessages.filter((m) => m.role === "user").pop();
	if (!lastUserMessage) {
		return {
			ok: false,
			status: 400,
			response: { error: "No user message found" },
		};
	}

	// Mark completion as started (atomically check and set)
	if (!startCompletion(completionId)) {
		// Race condition - another request started between our check and now
		const state = getCompletionState();
		log({
			event: "conflict_race",
			existingCompletionId: state.completionId,
		});
		return {
			ok: false,
			status: 409,
			response: {
				error: "completion_in_progress",
				completionId: state.completionId || "unknown",
			},
		};
	}

	log({ event: "started" });

	// Check for credential changes
	const {
		changed: credentialsChanged,
		env: credentialEnv,
		credentials: rawCredentials,
	} = checkCredentialsChanged(credentialsHeader);

	// Store agent and session references for cancellation
	currentAgent = agent;
	currentSessionId = sessionId;
	currentAbortController = new AbortController();

	// Run completion with abort signal in background (don't await)
	runCompletion(
		agent,
		completionId,
		lastUserMessage,
		credentialsChanged,
		credentialEnv,
		rawCredentials,
		gitUserName,
		gitUserEmail,
		model,
		reasoning,
		mode,
		log,
		sessionId,
		currentAbortController.signal,
		hookManager ?? null,
	);

	return {
		ok: true,
		status: 202,
		response: { completionId, status: "started" },
	};
}

export type CancelCompletionResult =
	| { ok: true; status: 200; response: CancelCompletionResponse }
	| { ok: false; status: 409; response: NoActiveCompletionResponse };

/**
 * Attempt to cancel an in-progress chat completion.
 * Returns a result object with the appropriate response and status code.
 */
export function tryCancelCompletion(): CancelCompletionResult {
	if (!isCompletionRunning()) {
		return {
			ok: false,
			status: 409,
			response: { error: "no_active_completion" },
		};
	}

	const state = getCompletionState();

	// Cancel through the agent interface (which will handle SDK-level cancellation)
	if (currentAgent) {
		currentAgent.cancel(currentSessionId).catch((err) => {
			console.error("Error calling agent.cancel():", err);
		});
	}

	// Also abort the controller as a fallback
	if (currentAbortController) {
		currentAbortController.abort();
		currentAbortController = null;
	}

	// Abort any background hook evaluation
	if (hookAbortController) {
		hookAbortController.abort();
		hookAbortController = null;
	}

	console.log(
		JSON.stringify({
			event: "cancelled",
			completionId: state.completionId,
		}),
	);

	return {
		ok: true,
		status: 200,
		response: {
			success: true,
			completionId: state.completionId || "unknown",
			status: "cancelled",
		},
	};
}

/**
 * Configure git user settings globally.
 * Runs git config commands to set user.name and user.email.
 */
async function configureGitUser(
	userName: string | null,
	userEmail: string | null,
): Promise<void> {
	if (userName) {
		await execAsync(`git config --global user.name "${userName}"`);
	}
	if (userEmail) {
		await execAsync(`git config --global user.email "${userEmail}"`);
	}
}

/**
 * Run a completion in the background. This function does not block -
 * it starts the completion and returns immediately. The completion
 * continues running even if the client disconnects.
 *
 * After the LLM turn completes, file hooks are evaluated non-blocking
 * in the background. If a hook fails with notify_llm, a new completion
 * is automatically triggered with the hook failure context.
 */
function runCompletion(
	agent: Agent,
	_completionId: string,
	lastUserMessage: UIMessage,
	credentialsChanged: boolean,
	credentialEnv: Record<string, string>,
	rawCredentials: CredentialEnvVar[],
	gitUserName: string | null,
	gitUserEmail: string | null,
	model: string | undefined,
	reasoning: "enabled" | "disabled" | "" | undefined,
	mode: "plan" | "" | undefined,
	log: (data: Record<string, unknown>) => void,
	sessionId: string | undefined,
	abortSignal: AbortSignal,
	hookManager: HookManager | null,
): void {
	// Run asynchronously without blocking the caller
	(async () => {
		// Clear any stale events from previous completions
		clearCompletionEvents();

		try {
			// Check if already cancelled before starting
			if (abortSignal.aborted) {
				throw new Error("Completion cancelled before start");
			}

			// Configure git user settings if provided
			if (gitUserName || gitUserEmail) {
				await configureGitUser(gitUserName, gitUserEmail);
			}

			// If credentials changed, update environment
			if (credentialsChanged) {
				await agent.updateEnvironment(credentialEnv, rawCredentials);
			}

			// Ensure connected and session exists BEFORE adding messages
			// (ensureSession may clear messages when creating a new session)
			if (!agent.isConnected) {
				await agent.connect();
			}
			await agent.ensureSession(sessionId);

			// Get the session
			const session = agent.getSession(sessionId);
			if (!session) {
				throw new Error("Failed to get or create session");
			}

			const message: UIMessage = {
				...lastUserMessage,
				id: lastUserMessage.id || generateMessageId(),
			};

			// Stream chunks from the agent's prompt generator
			for await (const chunk of agent.prompt(
				message,
				sessionId,
				model,
				reasoning,
				mode,
			)) {
				if (abortSignal.aborted) {
					addCompletionEvent({
						type: "finish",
						finishReason: "stop",
					});
					log({ event: "cancelled" });
					await finishCompletion();
					return;
				}

				addCompletionEvent(chunk);
			}

			log({ event: "completed" });
			await finishCompletion();
		} catch (error) {
			// Check if this was an abort error
			if (
				error instanceof Error &&
				(error.name === "AbortError" || error.message.includes("cancelled"))
			) {
				// Send finish event with stop reason for cancellation
				addCompletionEvent({
					type: "finish",
					finishReason: "stop",
				});
				log({ event: "cancelled" });
				await finishCompletion();
				return;
			}

			const errorText = extractErrorMessage(error);
			log({ event: "error", error: errorText });
			// Send error event to SSE stream so the client receives it
			addCompletionEvent({ type: "error", errorText });
			// Also send finish event with error reason for proper completion
			addCompletionEvent({
				type: "finish",
				finishReason: "error",
			});
			await finishCompletion(errorText);
		} finally {
			currentAgent = null;
			currentSessionId = undefined;
			currentAbortController = null;
		}

		// After completion finishes, evaluate file hooks non-blocking.
		// Parameters are still in closure scope after the finally block.
		if (hookManager?.hasFileHooks()) {
			scheduleHookEvaluation(
				agent,
				hookManager,
				model,
				reasoning,
				mode,
				sessionId,
			);
		}
	})();
}

/**
 * Schedule non-blocking hook evaluation after a completion finishes.
 *
 * Evaluates file hooks in the background. If a hook fails with notify_llm,
 * starts a new completion with the hook failure message. That completion
 * will in turn schedule another hook evaluation when it finishes, creating
 * a chain until all hooks pass or MAX_HOOK_RETRIES is reached.
 *
 * Aborted if the user starts a new completion (hookAbortController).
 */
function scheduleHookEvaluation(
	agent: Agent,
	hookManager: HookManager,
	model: string | undefined,
	reasoning: "enabled" | "disabled" | "" | undefined,
	mode: "plan" | "" | undefined,
	sessionId: string | undefined,
): void {
	hookAbortController = new AbortController();
	const signal = hookAbortController.signal;

	(async () => {
		// Grace period: let SSE handler flush final events and close
		// before potentially starting a new completion that clears them.
		// SSE polls every 50ms, so 200ms gives 4+ cycles.
		await new Promise((resolve) => setTimeout(resolve, 200));

		if (signal.aborted) return;

		const evalResult = await hookManager.evaluateFileHooks();

		if (signal.aborted) return;
		if (!evalResult.shouldReprompt) return;

		hookRetryCount++;
		if (hookRetryCount >= MAX_HOOK_RETRIES) {
			console.log(
				JSON.stringify({
					event: "hook_loop_guard",
					hookRetryCount,
					hookName: evalResult.failedResult?.hook.name,
				}),
			);
			return;
		}

		console.log(
			JSON.stringify({
				event: "hook_failed_starting_completion",
				hookRetryCount,
				hookName: evalResult.failedResult?.hook.name,
			}),
		);

		// Start a new completion with the hook failure context.
		// Uses tryStartCompletion directly — no resetHookState() so
		// hookRetryCount persists across the chain of hook retries.
		const hookMessage = createUIMessage("user", [
			{
				type: "text",
				text: evalResult.llmMessage || "A file hook failed.",
			},
		]);

		tryStartCompletion(
			agent,
			{ messages: [hookMessage] },
			null,
			null,
			null,
			model,
			reasoning,
			mode,
			sessionId,
			hookManager,
		);
	})();
}

/**
 * Extract error message from various error types (including JSON-RPC errors).
 */
function extractErrorMessage(error: unknown): string {
	if (error instanceof Error) {
		return error.message;
	}
	if (error && typeof error === "object") {
		const errorObj = error as Record<string, unknown>;
		if (typeof errorObj.message === "string") {
			let errorText = errorObj.message;
			// Include details from data.details if available (JSON-RPC format)
			if (errorObj.data && typeof errorObj.data === "object") {
				const data = errorObj.data as Record<string, unknown>;
				if (typeof data.details === "string") {
					errorText = `${errorText}: ${data.details}`;
				}
			}
			return errorText;
		}
	}
	return "Unknown error";
}
