import { access, constants } from "node:fs/promises";
import { delimiter, join as pathJoin } from "node:path";
import {
	type Options,
	query,
	type SDKMessage,
} from "@anthropic-ai/claude-agent-sdk";
import Anthropic from "@anthropic-ai/sdk";
import type { UIMessage, UIMessageChunk } from "ai";
import type { Agent } from "../agent/interface.js";
import type { ModelInfo } from "../api/types.js";
import {
	type AskUserQuestionInput,
	questionManager,
} from "../question-manager.js";
import {
	addCompletionEvent,
	loadSessionMapping,
	saveSessionMapping,
} from "../store/session.js";
import { messageToContentBlocks } from "./content-blocks.js";
import {
	type ClaudeSessionInfo,
	discoverSessions,
	getLastMessageError,
	loadSessionMessages,
} from "./persistence.js";
import {
	createTranslationState,
	type TranslationState,
	translateSDKMessage,
} from "./translate.js";

class SessionContext {
	nativeId: string | null;
	messages: UIMessage[];
	translationState: TranslationState | null;

	constructor(
		nativeId: string | null = null,
		messages: UIMessage[] = [],
		translationState: TranslationState | null = null,
	) {
		this.nativeId = nativeId;
		this.messages = messages;
		this.translationState = translationState;
	}

	async load(cwd: string) {
		if (!this.nativeId) {
			return;
		}
		this.messages = await loadSessionMessages(this.nativeId, cwd);
	}
}

export interface ClaudeSDKClientOptions {
	cwd: string;
	model?: string;
	env?: Record<string, string>;
}

/**
 * Find the Claude CLI binary on PATH or common installation locations
 */
async function findClaudeCLI(): Promise<string | null> {
	// First check environment variable override
	if (process.env.CLAUDE_CLI_PATH) {
		console.log(`[SDK] Using CLAUDE_CLI_PATH: ${process.env.CLAUDE_CLI_PATH}`);
		return process.env.CLAUDE_CLI_PATH;
	}

	const isWindows = process.platform === "win32";
	// On Windows, also check for .cmd and .exe extensions
	const binaryNames = isWindows
		? ["claude.cmd", "claude.exe", "claude"]
		: ["claude"];

	// Build list of paths to check (PATH + common locations)
	const pathsToCheck: string[] = [];

	// Add directories from PATH environment variable
	// Use path.delimiter (';' on Windows, ':' on Unix)
	if (process.env.PATH) {
		const pathDirs = process.env.PATH.split(delimiter);
		for (const dir of pathDirs) {
			if (dir) {
				for (const bin of binaryNames) {
					pathsToCheck.push(pathJoin(dir, bin));
				}
			}
		}
	}

	// Add common installation locations as fallback
	if (!isWindows) {
		const commonPaths = [
			process.env.HOME ? `${process.env.HOME}/.local/bin/claude` : null,
			"/usr/bin/claude",
			"/usr/local/bin/claude",
			"/opt/homebrew/bin/claude",
		].filter(Boolean) as string[];

		for (const commonPath of commonPaths) {
			if (!pathsToCheck.includes(commonPath)) {
				pathsToCheck.push(commonPath);
			}
		}
	}

	// Try each path in order
	for (const path of pathsToCheck) {
		try {
			// On Windows, check existence only (X_OK is not reliable)
			await access(path, isWindows ? constants.F_OK : constants.X_OK);
			console.log(`[SDK] Found Claude CLI at: ${path}`);
			return path;
		} catch {
			// Not found or not executable at this path, try next
		}
	}

	console.warn(
		"[SDK] Could not find Claude CLI. Set CLAUDE_CLI_PATH environment variable or ensure 'claude' is on PATH.",
	);
	console.warn(`[SDK] Searched ${pathsToCheck.length} locations`);
	console.warn(`[SDK] Current PATH: ${process.env.PATH}`);
	console.warn(`[SDK] Current HOME: ${process.env.HOME}`);
	return null;
}

export class ClaudeSDKClient implements Agent {
	private sessions = new Map<string, SessionContext>();
	private env: Record<string, string>;
	private claudeCliPath: string | null = null;
	private cwd: string;
	private setup = false;
	private activeAbortController: AbortController | null = null;

	constructor(private options: ClaudeSDKClientOptions) {
		const REDACTED_ENV_KEYS = ["ANTHROPIC_API_KEY", "CLAUDE_CODE_OAUTH_TOKEN"];
		const redactedEnv = Object.fromEntries(
			Object.entries(options.env ?? {}).map(([k, v]) =>
				REDACTED_ENV_KEYS.includes(k) ? [k, "[REDACTED]"] : [k, v],
			),
		);
		console.log("ClaudeSDKClient constructor", {
			...options,
			env: redactedEnv,
		});
		this.env = { ...options.env };
		this.cwd = options.cwd;
	}

	private async ensureSetup(): Promise<void> {
		if (!this.setup) {
			this.claudeCliPath = await findClaudeCLI();
			if (!this.claudeCliPath) {
				throw new Error(
					"Claude CLI not found. Install it or set CLAUDE_CLI_PATH environment variable.",
				);
			}
			this.setup = true;
		}
	}

	async ensureSession(sessionId: string): Promise<SessionContext> {
		let ctx = this.sessions.get(sessionId);
		if (ctx) {
			if (!ctx.nativeId) {
				try {
					ctx.nativeId = await loadSessionMapping(sessionId);
				} catch {
					// ignore
				}
			}
			return ctx;
		}

		const existingSessions = await this.discoverAvailableSessions();
		if (existingSessions.length === 1) {
			try {
				const nativeId = existingSessions[0].sessionId;
				const messages = await loadSessionMessages(nativeId, this.cwd);
				await saveSessionMapping(sessionId, nativeId);
				ctx = new SessionContext(nativeId, messages);
			} catch {
				// ignore existing session
			}
		}

		if (!ctx) {
			ctx = new SessionContext();
		}
		this.sessions.set(sessionId, ctx);
		return ctx;
	}

	/**
	 * Discover all available sessions from ~/.claude
	 */
	async discoverAvailableSessions(): Promise<ClaudeSessionInfo[]> {
		return discoverSessions(this.options.cwd);
	}

	async *prompt(
		message: UIMessage,
		sessionId: string,
		model?: string,
		reasoning?: "enabled" | "disabled" | "",
		mode?: "plan" | "",
	): AsyncGenerator<UIMessageChunk, void, unknown> {
		await this.ensureSetup();
		const ctx = await this.ensureSession(sessionId);

		// Reload messages from disk at start of each turn.
		await ctx.load(this.cwd);

		// Initialize translation state for this prompt (will be set properly on message_start)
		ctx.translationState = null;

		// Convert message parts to Claude SDK content blocks format
		// This includes text and image attachments
		const contentBlocks = messageToContentBlocks(message);

		// Create abort controller for this prompt
		this.activeAbortController = new AbortController();

		// Trim "anthropic:" prefix from model if present (models are stored as provider:model-id)
		let sdkModel = model || this.options.model;
		if (sdkModel?.startsWith("anthropic:")) {
			sdkModel = sdkModel.substring("anthropic:".length);
		}

		// Configure thinking options based on reasoning parameter
		// The Claude Agent SDK uses maxThinkingTokens (not the raw API's adaptive thinking format)
		let thinkingOptions = {};
		if (reasoning === "enabled") {
			thinkingOptions = { maxThinkingTokens: 10000 };
		}

		// Log prompt dispatch details for debugging
		const textPreview = contentBlocks
			.filter((b): b is { type: "text"; text: string } => b.type === "text")
			.map((b) => b.text)
			.join("")
			.slice(0, 100);
		console.log("[SDK] prompt →", {
			model: sdkModel ?? "(default)",
			reasoning,
			mode: mode || "(default/build)",
			thinkingOptions,
			text: textPreview + (textPreview.length === 100 ? "…" : ""),
		});

		// Get the native Claude CLI session ID for resume.
		// On first prompt this is null (Claude creates a new session), on subsequent
		// prompts it's the ID captured from the init message.
		const resumeId = ctx.nativeId ?? undefined;
		const sdkOptions: Options = {
			cwd: this.options.cwd,
			model: sdkModel,
			resume: resumeId,
			env: this.env,
			includePartialMessages: true,
			tools: { type: "preset", preset: "claude_code" },
			systemPrompt: { type: "preset", preset: "claude_code" },
			settingSources: ["user", "project"], // Load user settings from ~/.claude and CLAUDE.md files
			// Permission mode: 'plan' for planning (no tool execution), undefined for default (build)
			...(mode === "plan" ? { permissionMode: "plan" as const } : {}),
			// Apply thinking options (adaptive for Opus 4.6, maxThinkingTokens for older models)
			...thinkingOptions,
			// Use the discovered Claude CLI path from connect()
			pathToClaudeCodeExecutable: this.claudeCliPath ?? "",
			// Pass abort controller to SDK for proper cancellation
			abortController: this.activeAbortController,
			// Use canUseTool to intercept tool calls that require user interaction:
			// - AskUserQuestion: emit question chunk, block until user answers via POST /chat/answer
			// - ExitPlanMode: emit approval request, block until user approves/rejects, emit mode-change
			// Note: EnterPlanMode is handled in the prompt() generator (CLI auto-approves it).
			// All other tools are auto-approved.
			canUseTool: async (toolName, input, options) => {
				console.log(
					`[SDK] canUseTool: ${toolName}, toolUseID: ${options.toolUseID}`,
				);

				if (toolName === "AskUserQuestion") {
					const questionInput = input as unknown as AskUserQuestionInput;

					// Register the question in QuestionManager BEFORE emitting the chunk,
					// so the GET /chat/question endpoint can return it immediately when
					// the frontend queries after seeing the approval-requested state.
					const answerPromise = questionManager.waitForAnswer(
						options.toolUseID,
						questionInput.questions,
					);

					// Yield to event loop so the completion runner can process
					// the tool-input-start/delta/available chunks from the assistant
					// message that the SDK enqueued before calling canUseTool.
					// Without this, tool-approval-request arrives before tool-input-start,
					// causing getToolInvocation() to throw UIMessageStreamError.
					await new Promise((resolve) => setTimeout(resolve, 100));

					// NOW emit the approval request (tool part exists in the frontend)
					addCompletionEvent({
						type: "tool-approval-request",
						toolCallId: options.toolUseID,
						approvalId: options.toolUseID,
					} as unknown as UIMessageChunk);

					console.log(
						`[SDK] AskUserQuestion: emitted tool-approval-request, waiting for user answer (toolUseID: ${options.toolUseID})`,
					);

					// Block until the frontend submits an answer.
					// Respect the SDK's abort signal for clean cancellation.
					const answers = await new Promise<Record<string, string>>(
						(resolve, reject) => {
							answerPromise.then(resolve, reject);
							if (options.signal.aborted) {
								questionManager.cancelAll("Tool permission request aborted");
								reject(new Error("Aborted"));
								return;
							}
							options.signal.addEventListener(
								"abort",
								() => {
									questionManager.cancelAll("Tool permission request aborted");
									reject(new Error("Aborted"));
								},
								{ once: true },
							);
						},
					);

					console.log(
						`[SDK] AskUserQuestion: received answer for toolUseID ${options.toolUseID}`,
					);

					return {
						behavior: "allow" as const,
						updatedInput: {
							questions: questionInput.questions,
							answers,
						} as unknown as Record<string, unknown>,
						toolUseID: options.toolUseID,
					};
				}

				// Handle ExitPlanMode: block for user approval, then emit mode-change event.
				// This mirrors AskUserQuestion: emit approval-request, wait for answer, then continue.
				if (toolName === "ExitPlanMode") {
					const exitInput = input as {
						plan?: string;
						allowedPrompts?: { tool: string; prompt: string }[];
					};

					// Register the approval request in QuestionManager using the plan approval
					// as a question. The frontend shows the plan and an approve/reject button.
					const answerPromise = questionManager.waitForAnswer(
						options.toolUseID,
						[
							{
								question:
									"The agent has finished planning and wants to start implementing. Approve to exit plan mode and switch to build mode.",
								header: "Plan",
								options: [
									{
										label: "Approve",
										description: "Exit plan mode and start building",
									},
									{
										label: "Reject",
										description: "Stay in plan mode",
									},
								],
								multiSelect: false,
							},
						],
						exitInput.plan,
					);

					// Yield to event loop (same ordering fix as AskUserQuestion above)
					await new Promise((resolve) => setTimeout(resolve, 100));

					addCompletionEvent({
						type: "tool-approval-request",
						toolCallId: options.toolUseID,
						approvalId: options.toolUseID,
					} as unknown as UIMessageChunk);

					console.log(
						`[SDK] ExitPlanMode: emitted tool-approval-request, waiting for user approval (toolUseID: ${options.toolUseID})`,
					);

					// Block until the frontend submits an answer.
					// Respect the SDK's abort signal for clean cancellation.
					const answers = await new Promise<Record<string, string>>(
						(resolve, reject) => {
							answerPromise.then(resolve, reject);
							if (options.signal.aborted) {
								questionManager.cancelAll("Tool permission request aborted");
								reject(new Error("Aborted"));
								return;
							}
							options.signal.addEventListener(
								"abort",
								() => {
									questionManager.cancelAll("Tool permission request aborted");
									reject(new Error("Aborted"));
								},
								{ once: true },
							);
						},
					);

					console.log(
						`[SDK] ExitPlanMode: received answer for toolUseID ${options.toolUseID}`,
					);

					// Check if user approved (first question answer is "Approve")
					const approved = Object.values(answers).some((v) => v === "Approve");

					if (!approved) {
						return {
							behavior: "deny" as const,
							message: "User declined to exit plan mode. Continue planning.",
							toolUseID: options.toolUseID,
						};
					}

					// Emit transient data chunk to flip frontend mode and update session
					addCompletionEvent({
						type: "data-mode-change",
						data: { mode: "" },
						transient: true,
					} as unknown as UIMessageChunk);

					return {
						behavior: "allow" as const,
						updatedInput: {
							...exitInput,
							allowedPrompts: exitInput.allowedPrompts,
						} as unknown as Record<string, unknown>,
						toolUseID: options.toolUseID,
					};
				}

				// Note: EnterPlanMode is NOT handled here. The CLI auto-approves it
				// internally without sending a can_use_tool control request, so canUseTool
				// is never called for it. Detection is in the prompt() generator loop instead.

				// Auto-approve all other tools
				// Note: updatedInput is required by the SDK's runtime Zod schema even though TypeScript marks it optional
				return {
					behavior: "allow" as const,
					updatedInput: input,
					toolUseID: options.toolUseID,
				};
			},
			// Tool outputs are handled through streaming events (tool_result blocks)
			// rather than PostToolUse hooks to maintain proper event ordering
		};

		const promptGenerator = (async function* () {
			yield {
				type: "user" as const,
				message: {
					role: "user" as const,
					content: contentBlocks,
				},
				parent_tool_use_id: null,
				session_id: resumeId ?? "",
			};
		})();

		// Helper function to check last message for errors and create error chunks
		const checkLastMessageError = async (): Promise<{
			errorMessage: string | null;
			chunks: UIMessageChunk[];
		}> => {
			const ctx = await this.ensureSession(sessionId);
			const errorMessage = ctx.nativeId
				? await getLastMessageError(ctx.nativeId, this.options.cwd)
				: null;

			if (!errorMessage) {
				return { errorMessage: null, chunks: [] };
			}

			// Create error chunks
			const chunks: UIMessageChunk[] = [
				{ type: "error", errorText: errorMessage } as UIMessageChunk,
				{ type: "finish", finishReason: "error" } as UIMessageChunk,
			];

			return { errorMessage, chunks };
		};

		// Start query and yield chunks
		const q = query({ prompt: promptGenerator, options: sdkOptions });

		try {
			for await (const sdkMsg of q) {
				const chunks = this.translateSDKMessage(sessionId, ctx, sdkMsg);
				for (const chunk of chunks) {
					yield chunk;

					// Detect EnterPlanMode in the message stream.
					// canUseTool is never called for this tool because the CLI
					// auto-approves it internally (entering plan mode is a transition
					// to a more restrictive mode), so we detect it here instead.
					if (
						chunk.type === "tool-input-start" &&
						"toolName" in chunk &&
						chunk.toolName === "EnterPlanMode"
					) {
						yield {
							type: "data-mode-change",
							data: { mode: "plan" },
							transient: true,
						} as unknown as UIMessageChunk;
					}
				}
			}

			// After the query completes successfully, check if an error was written to the messages file
			// with explicit error fields (edge case where SDK doesn't throw)
			const { errorMessage, chunks } = await checkLastMessageError();
			if (errorMessage) {
				console.error(`[SDK] Detected error in last message: ${errorMessage}`);
				// Yield error chunks
				for (const chunk of chunks) {
					yield chunk;
				}
				throw new Error(errorMessage);
			}
		} catch (error) {
			// Check if this is a process exit error - if so, try to get the user-friendly
			// error message from the last message on disk instead of the generic exit code
			if (
				error instanceof Error &&
				error.message.includes("process exited with code")
			) {
				console.log(
					`[SDK] Claude process exited, checking last message for user-friendly error...`,
				);

				const { errorMessage, chunks } = await checkLastMessageError();
				if (errorMessage) {
					console.log(`[SDK] Found user-friendly error: ${errorMessage}`);
					// Yield error chunks
					for (const chunk of chunks) {
						yield chunk;
					}
					// Throw the user-friendly error instead of the generic process exit error
					throw new Error(errorMessage);
				}
			}

			// Re-throw the original error if we couldn't find a better message
			throw error;
		} finally {
			this.activeAbortController = null;
			// Reload messages from disk after prompt completes
			await ctx.load(this.cwd);
		}
	}

	async cancel(sessionId: string): Promise<void> {
		if (this.activeAbortController) {
			console.log("[SDK] Cancelling active prompt via abortController");

			// Cancel any pending AskUserQuestion before aborting the SDK query.
			// This causes the canUseTool promise to reject with a "cancelled" error
			// which propagates as a clean stop through the completion runner.
			questionManager.cancelAll();

			// Abort the SDK query - the SDK will clean up resources properly
			this.activeAbortController.abort();
			this.activeAbortController = null;

			// Clear translation state but keep session history
			const ctx = this.sessions.get(sessionId);
			if (ctx) {
				ctx.translationState = null;
			}
		}
	}

	async getMessages(sessionId: string): Promise<UIMessage[]> {
		await this.ensureSetup();
		const ctx = await this.ensureSession(sessionId);
		return ctx.messages;
	}

	async updateEnvironment(
		_sessionId: string,
		update: Record<string, string>,
	): Promise<void> {
		Object.assign(this.env, update);
	}

	getEnvironment(): Record<string, string> {
		return { ...this.env };
	}

	/**
	 * Determine if a model supports extended thinking/reasoning based on its ID.
	 * Models that support extended thinking:
	 * - Claude Opus 4.x (claude-opus-4*)
	 * - Claude Sonnet 4.x (claude-sonnet-4*)
	 * - Models with "thinking" in the name
	 * Models that do NOT support it:
	 * - Claude Haiku (all versions)
	 * - Claude Sonnet 3.x and earlier
	 */
	private supportsReasoning(modelId: string): boolean {
		const lowerModelId = modelId.toLowerCase();

		// Haiku never supports extended thinking
		if (lowerModelId.includes("haiku")) {
			return false;
		}

		// Models with "thinking" in the name support it
		if (lowerModelId.includes("thinking")) {
			return true;
		}

		// Claude Opus 4.x supports extended thinking
		if (lowerModelId.includes("opus") && lowerModelId.includes("4")) {
			return true;
		}

		// Claude Sonnet 4.x supports extended thinking (but not 3.x)
		if (lowerModelId.includes("sonnet")) {
			// Check for version 4 or higher (4.0, 4.5, etc.)
			const match = lowerModelId.match(/sonnet[- ]?(\d+)/);
			if (match) {
				const version = Number.parseInt(match[1], 10);
				return version >= 4;
			}
		}

		// Default to false for unknown models
		return false;
	}

	async listModels(_sessionId: string): Promise<ModelInfo[]> {
		await this.ensureSetup();
		// Check for OAuth token vs API key
		const oauthToken = this.env.CLAUDE_CODE_OAUTH_TOKEN;
		const apiKey = this.env.ANTHROPIC_API_KEY;

		if (!oauthToken && !apiKey) {
			throw new Error(
				"ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN not configured",
			);
		}

		// Create Anthropic client with proper authentication
		const clientOptions: ConstructorParameters<typeof Anthropic>[0] = {};

		if (oauthToken) {
			// OAuth: Use Bearer authentication (authToken)
			clientOptions.authToken = oauthToken;
		} else {
			// API Key: Use x-api-key authentication
			clientOptions.apiKey = apiKey;
		}

		const client = new Anthropic(clientOptions);

		try {
			// Call the models API
			// Use beta.models.list() with betas param for OAuth, regular models.list() for API keys
			const response = oauthToken
				? await client.beta.models.list({
						betas: ["oauth-2025-04-20"],
					})
				: await client.models.list();

			// Map to our ModelInfo type with "anthropic:" prefix and reasoning detection
			return response.data.map((model) => ({
				id: `anthropic:${model.id}`,
				display_name: model.display_name || model.id,
				provider: "Anthropic",
				created_at: model.created_at,
				type: model.type,
				reasoning: this.supportsReasoning(model.id),
			}));
		} catch (error) {
			console.error("Failed to list models from Anthropic API:", error);
			throw error;
		}
	}

	/**
	 * Translate an SDK message to UIMessageChunks.
	 * Also handles session ID capture and translation state management.
	 */
	private translateSDKMessage(
		sessionId: string,
		ctx: SessionContext,
		msg: SDKMessage,
	): UIMessageChunk[] {
		// Capture the native Claude CLI session ID from the init message so
		// subsequent prompts can resume it via the session's nativeId.
		if (msg.type === "system" && msg.subtype === "init") {
			if (msg.session_id) {
				console.log(
					`[SDK] Init message: storing native session ID ${msg.session_id}`,
				);
				saveSessionMapping(sessionId, msg.session_id)
					.then(() => {
						ctx.nativeId = msg.session_id;
					})
					.catch((err) => {
						console.error("[SDK] Failed to persist session mapping:", err);
					});
			}
			return [];
		}

		// Initialize translation state if needed (only once per prompt, not per message)
		// The translate module manages the state across multiple messages in an agentic loop
		if (!ctx.translationState) {
			ctx.translationState = createTranslationState("");
		}

		// Translate SDK message to UIMessageChunks
		const chunks = translateSDKMessage(msg, ctx.translationState);

		// Clean up translation state on result
		if (msg.type === "result") {
			ctx.translationState = null;
		}

		return chunks;
	}
}
