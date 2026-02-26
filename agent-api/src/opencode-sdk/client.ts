/**
 * OpenCode SDK Client Implementation
 *
 * This file implements the Agent interface using the OpenCode SDK.
 */

import {
	createOpencode,
	type Event as OpenCodeEvent,
	type OpencodeClient,
} from "@opencode-ai/sdk/v2";
import type { UIMessage, UIMessageChunk } from "ai";
import type { Agent } from "../agent/interface.js";
import type { ModelInfo } from "../api/types.js";
import type { CredentialEnvVar } from "../credentials/credentials.js";
import { questionManager } from "../question-manager.js";
import { loadSessionMapping, saveSessionMapping } from "../store/session.js";
import {
	createStreamTranslationState,
	translateEventsToChunks,
	translateOpenCodeMessageToUIMessage,
	translateUIMessageToParts,
} from "./translate.js";

class SessionContext {
	nativeId: string | null;
	messages: UIMessage[];

	constructor(nativeId: string | null = null, messages: UIMessage[] = []) {
		this.nativeId = nativeId;
		this.messages = messages;
	}

	async load(client: OpencodeClient): Promise<void> {
		if (!this.nativeId) return;
		const result = await client.session.messages({ sessionID: this.nativeId });
		if (result.error || !result.data) return;
		console.log(
			`[opencode] session.load: ${result.data.length} messages from OpenCode`,
		);
		this.messages = result.data
			.map((msg) => translateOpenCodeMessageToUIMessage(msg.info, msg.parts))
			.filter((msg) => msg.parts.length > 0);
	}
}

/**
 * OpenCodeClient implements the Agent interface using the OpenCode SDK.
 *
 * Architecture:
 * - Creates an OpenCode server instance (or connects to existing one)
 * - Multi-session model: each discobot session ID maps to a native OpenCode session
 * - Translates UIMessage format to OpenCode parts format
 * - Streams responses via OpenCode event system
 * - Translates OpenCode events to UIMessageChunks
 */
export class OpenCodeClient implements Agent {
	private connected = false;
	private client: OpencodeClient | null = null;
	private serverClose: (() => void) | null = null;
	private env: Record<string, string>;
	private cwd: string;
	private model?: string;
	private providerID: string;
	private modelID: string;

	// Session management - keyed by discobot session ID
	private sessions: Map<string, SessionContext> = new Map();

	// Cancellation support
	private activeAbortControllers: Map<string, AbortController> = new Map();

	// Event stream for permission auto-approval and question forwarding
	private eventStream: AsyncGenerator<OpenCodeEvent> | null = null;
	private eventLoopRunning = false;

	// Callback for routing events to the active prompt() generator
	private promptEventCallback: ((event: OpenCodeEvent) => void) | null = null;

	constructor(options: OpenCodeClientOptions) {
		this.cwd = options.cwd || process.cwd();
		this.model = options.model;
		this.env = options.env || {};

		// Parse model string to extract providerID and modelID
		// Default to opencode/big-pickle if not specified
		if (this.model) {
			// Model format could be "big-pickle" or "opencode/big-pickle"
			const parts = this.model.split("/");
			if (parts.length === 2) {
				this.providerID = parts[0];
				this.modelID = parts[1];
			} else {
				// Default to opencode provider
				this.providerID = "opencode";
				this.modelID = this.model;
			}
		} else {
			// Default model
			this.providerID = "opencode";
			this.modelID = "big-pickle";
		}
	}

	async connect(): Promise<void> {
		if (this.connected) {
			return;
		}

		const { client, server } = await createOpencode({
			config: {},
		});

		this.client = client;
		this.serverClose = server.close;
		this.connected = true;

		// Start event listener for permission auto-approval and question forwarding
		this.startEventLoop();
	}

	async disconnect(): Promise<void> {
		if (!this.connected) {
			return;
		}

		// Stop event loop
		this.stopEventLoop();

		// Cancel any pending questions
		questionManager.cancelAll();

		// Cancel any active operations
		for (const [sessionId] of this.activeAbortControllers) {
			await this.cancel(sessionId);
		}

		// Close the server via the SDK
		if (this.serverClose) {
			this.serverClose();
			this.serverClose = null;
		}

		this.client = null;
		this.connected = false;
		this.sessions.clear();
	}

	private async ensureConnected(): Promise<OpencodeClient> {
		if (!this.connected) {
			await this.connect();
		}
		if (!this.client) {
			throw new Error("OpenCode client is not connected");
		}
		return this.client;
	}

	async ensureSession(sessionId: string): Promise<SessionContext> {
		const client = await this.ensureConnected();

		let ctx = this.sessions.get(sessionId);
		if (ctx) {
			if (!ctx.nativeId) {
				try {
					const nativeId = await loadSessionMapping(sessionId);
					if (nativeId) {
						ctx.nativeId = nativeId;
						await ctx.load(client);
					}
				} catch {
					// ignore
				}
			}
			return ctx;
		}

		// Try to restore from persisted mapping
		try {
			const nativeId = await loadSessionMapping(sessionId);
			if (nativeId) {
				// Verify the OpenCode session still exists
				const sessionResult = await client.session.get({ sessionID: nativeId });
				if (!sessionResult.error) {
					ctx = new SessionContext(nativeId);
					await ctx.load(client);
				}
			}
		} catch {
			// ignore
		}

		// Create a new OpenCode session if we don't have one
		if (!ctx) {
			const result = await client.session.create({
				// Auto-approve all tool permissions so the agent doesn't block
				permission: [{ permission: "*", pattern: "*", action: "allow" }],
			});

			if (result.error) {
				throw new Error(`Failed to create OpenCode session: ${result.error}`);
			}

			const nativeId = result.data.id;
			await saveSessionMapping(sessionId, nativeId);
			ctx = new SessionContext(nativeId);
		}

		this.sessions.set(sessionId, ctx);
		return ctx;
	}

	async *prompt(
		message: UIMessage,
		sessionId: string,
		model?: string,
		_reasoning?: "enabled" | "disabled" | "",
		_mode?: "plan" | "",
	): AsyncGenerator<UIMessageChunk, void, unknown> {
		const client = await this.ensureConnected();
		const ctx = await this.ensureSession(sessionId);

		if (!ctx.nativeId) {
			throw new Error("OpenCode session ID is not set after session creation");
		}

		const opencodeSessionId = ctx.nativeId;

		// Resolve model for this request
		// Model can come from: per-request param > constructor option
		// Format may be "anthropic:claude-sonnet-4-5-20250929" or just "claude-sonnet-4-5-20250929"
		const requestModel = model || this.model;
		const { providerID, modelID } = this.parseModel(requestModel);

		// Translate UIMessage to OpenCode parts
		const parts = translateUIMessageToParts(message);

		// Ensure message ID has the required "msg_" prefix for OpenCode
		const messageID = message.id.startsWith("msg_")
			? message.id
			: `msg_${message.id}`;

		// Create abort controller for cancellation
		const abortController = new AbortController();
		this.activeAbortControllers.set(sessionId, abortController);

		// Event-driven streaming queue
		const queue: UIMessageChunk[] = [];
		let notify: (() => void) | null = null;
		let done = false;

		// The assistant message ID created in response to our prompt.
		// We only start emitting chunks once we identify it via parentID.
		let responseMessageId: string | null = null;

		// Stateful translation: tracks assistant message ID and started parts
		// so we can properly sequence text-start/text-end around deltas,
		// filter user message parts, and use real step events.
		const translationState = createStreamTranslationState();

		// Register callback to receive events from the event loop
		this.promptEventCallback = (event) => {
			// Handle abort
			if (abortController.signal.aborted) {
				console.log(`[opencode] → aborted`);
				done = true;
				notify?.();
				return;
			}

			// Detect our response message via parentID.
			// The assistant message created in response to our prompt will
			// have parentID === messageID.
			if (event.type === "message.updated") {
				const info = event.properties.info;
				if (
					info.sessionID === opencodeSessionId &&
					info.role === "assistant" &&
					"parentID" in info &&
					info.parentID === messageID
				) {
					if (!responseMessageId) {
						responseMessageId = info.id;
						console.log(
							`[opencode] → response message ${info.id} (parentID=${messageID})`,
						);
					}
					if (info.time.completed) {
						console.log(`[opencode] → done (message completed)`);
						if (info.error) {
							const errorMsg =
								"message" in info.error
									? (info.error.message as string)
									: "data" in info.error &&
											typeof info.error.data === "object" &&
											info.error.data &&
											"message" in info.error.data
										? String(info.error.data.message)
										: "Agent error";
							queue.push({ type: "error", errorText: errorMsg });
						}
						done = true;
						// Don't return — let the event go through translation
						// so it emits the finish chunk.
					}
				}
			}

			// session.error is a fallback for errors that aren't on the message
			if (event.type === "session.error") {
				const props = event.properties as {
					sessionID?: string;
					error?: { name: string; data: { message: string } };
				};
				if (!props.sessionID || props.sessionID === opencodeSessionId) {
					const errorMsg =
						props.error?.data?.message ?? "Unknown session error";
					console.log(`[opencode] → error: ${errorMsg}`);
					queue.push({ type: "error", errorText: errorMsg });
					done = true;
					notify?.();
					return;
				}
			}

			// Don't emit any chunks until we've identified our response message
			if (!responseMessageId) {
				return;
			}

			// Translate message events to UIMessageChunks (stateful)
			const chunks = translateEventsToChunks(
				event,
				opencodeSessionId,
				translationState,
			);
			if (chunks.length === 0) {
				if (done) notify?.();
				return;
			}

			// Add model metadata to start chunks
			for (const chunk of chunks) {
				if (chunk.type === "start" && !chunk.messageMetadata) {
					chunk.messageMetadata = {
						model: `${providerID}:${modelID}`,
					};
				}
			}

			console.log(`[opencode] → chunks: ${JSON.stringify(chunks)}`);

			queue.push(...chunks);
			notify?.();
		};

		try {
			// Fire prompt asynchronously — returns 204 immediately
			console.log(
				`[opencode] promptAsync sessionID=${opencodeSessionId} messageID=${messageID} model=${providerID}:${modelID}`,
			);
			const promptResult = await client.session.promptAsync({
				sessionID: opencodeSessionId,
				messageID,
				parts,
				model: {
					providerID,
					modelID,
				},
			});

			if (promptResult.error) {
				throw new Error(
					`OpenCode promptAsync failed: ${JSON.stringify(promptResult.error)}`,
				);
			}
			console.log("[opencode] promptAsync accepted, streaming events...");

			// Consume queue until the session goes idle
			while (!done || queue.length > 0) {
				// Wait for events if queue is empty
				if (queue.length === 0 && !done) {
					await new Promise<void>((r) => {
						notify = r;
					});
					notify = null;
				}

				// Yield all queued chunks
				while (queue.length > 0) {
					const chunk = queue.shift();
					if (chunk) {
						yield chunk;
					}
				}
			}

			console.log("[opencode] streaming complete, refreshing session cache");
			await ctx.load(client);
		} finally {
			this.promptEventCallback = null;
			this.activeAbortControllers.delete(sessionId);
		}
	}

	/**
	 * Parse a model string into providerID and modelID.
	 * Handles formats: "anthropic:model-id", "provider/model-id", "model-id"
	 */
	private parseModel(model?: string): {
		providerID: string;
		modelID: string;
	} {
		if (!model) {
			return { providerID: this.providerID, modelID: this.modelID };
		}

		// Handle "provider:model" format (e.g. "anthropic:claude-sonnet-4-5-20250929")
		if (model.includes(":")) {
			const [provider, ...rest] = model.split(":");
			return { providerID: provider, modelID: rest.join(":") };
		}

		// Handle "provider/model" format
		if (model.includes("/")) {
			const [provider, ...rest] = model.split("/");
			return { providerID: provider, modelID: rest.join("/") };
		}

		// Just a model ID, use default provider
		return { providerID: this.providerID, modelID: model };
	}

	async cancel(sessionId: string): Promise<void> {
		// Abort via controller
		const controller = this.activeAbortControllers.get(sessionId);
		if (controller) {
			controller.abort();
			this.activeAbortControllers.delete(sessionId);
		}

		// Cancel any pending questions
		questionManager.cancelAll();

		// Abort via OpenCode API
		const ctx = this.sessions.get(sessionId);
		const nativeId = ctx?.nativeId;
		if (nativeId && this.client) {
			await this.client.session.abort({ sessionID: nativeId });
		}
	}

	async updateEnvironment(
		_sessionId: string,
		update: Record<string, string>,
		credentials?: CredentialEnvVar[],
	): Promise<void> {
		this.env = { ...this.env, ...update };

		// Push credentials to the running OpenCode server via auth.set() API.
		// Env vars alone don't reach the already-spawned server process.
		if (this.connected && this.client && credentials) {
			for (const cred of credentials) {
				try {
					const auth =
						cred.authType === "oauth"
							? {
									type: "oauth" as const,
									access: cred.value,
									refresh: "",
									expires: cred.expiresAt ?? 0,
								}
							: { type: "api" as const, key: cred.value };

					console.log(
						`[opencode] auth.set provider=${cred.provider} type=${auth.type}`,
					);
					await this.client.auth.set({
						providerID: cred.provider,
						auth,
					});
				} catch (err) {
					console.error(
						`[opencode] Failed to set auth for provider ${cred.provider}:`,
						err,
					);
				}
			}
		}
	}

	getEnvironment(): Record<string, string> {
		return { ...this.env };
	}

	async listModels(_sessionId: string): Promise<ModelInfo[]> {
		const client = await this.ensureConnected();

		const result = await client.config.providers({
			directory: this.cwd,
		});

		if (result.error || !result.data) {
			throw new Error(`Failed to list models: ${result.error}`);
		}

		const models: ModelInfo[] = [];

		for (const provider of result.data.providers) {
			for (const [modelId, model] of Object.entries(provider.models)) {
				models.push({
					id: `${provider.id}:${modelId}`,
					display_name: model.name || modelId,
					provider: provider.name || provider.id,
					created_at: new Date().toISOString(),
					type: "model",
					reasoning: model.capabilities?.reasoning ?? false,
				});
			}
		}

		return models;
	}

	async getMessages(sessionId: string): Promise<UIMessage[]> {
		const ctx = await this.ensureSession(sessionId);
		return ctx.messages;
	}

	async clearSession(sessionId: string): Promise<void> {
		const ctx = this.sessions.get(sessionId);

		// Cancel any pending questions
		questionManager.cancelAll();

		if (ctx) {
			// Delete OpenCode session
			const nativeId = ctx.nativeId;
			if (nativeId && this.client) {
				await this.client.session.delete({ sessionID: nativeId });
			}

			// Clear local session cache
			ctx.messages = [];
		}
	}

	/**
	 * Start the SSE event loop that auto-approves permissions and forwards questions.
	 */
	private startEventLoop(): void {
		if (!this.client || this.eventLoopRunning) return;
		this.eventLoopRunning = true;

		const client = this.client;
		(async () => {
			try {
				const result = await client.event.subscribe();
				const stream = result.stream;
				this.eventStream = stream;

				for await (const event of stream) {
					if (!this.eventLoopRunning) break;
					this.handleEvent(event, client);
				}
			} catch {
				// Event stream closed (expected on disconnect)
			} finally {
				this.eventLoopRunning = false;
				this.eventStream = null;
			}
		})();
	}

	/**
	 * Stop the SSE event loop.
	 */
	private stopEventLoop(): void {
		this.eventLoopRunning = false;
		if (this.eventStream) {
			this.eventStream.return(undefined);
			this.eventStream = null;
		}
	}

	/**
	 * Handle a single SSE event from the OpenCode server.
	 */
	private handleEvent(event: OpenCodeEvent, client: OpencodeClient): void {
		switch (event.type) {
			case "permission.asked": {
				// Auto-approve all permission requests.
				// This is a fallback — session-level permissions should handle most cases,
				// but some permissions may still be asked at runtime.
				const { id: requestID } = event.properties;
				client.permission.reply({ requestID, reply: "always" }).catch(() => {
					// Ignore errors — server may be shutting down
				});
				break;
			}
			case "question.asked": {
				// Forward the question to the QuestionManager so the frontend
				// can display it and the user can answer.
				const req = event.properties;
				const questions = req.questions.map((q) => ({
					question: q.question,
					header: q.header,
					options: q.options.map((o) => ({
						label: o.label,
						description: o.description,
					})),
					multiSelect: q.multiple ?? false,
				}));

				// Register the question and wait for the user's answer
				questionManager
					.waitForAnswer(req.id, questions)
					.then((answers) => {
						// Translate answers from Record<string, string> to Array<Array<string>>
						// QuestionManager gives us { "0": "Yes", "1": "Option A" }
						// OpenCode expects [[selected labels per question]]
						const answerArrays = Object.entries(answers)
							.sort(([a], [b]) => Number(a) - Number(b))
							.map(([, value]) => [value]);

						return client.question.reply({
							requestID: req.id,
							answers: answerArrays,
						});
					})
					.catch(() => {
						// Question was cancelled — reject it so the agent can continue
						client.question.reject({ requestID: req.id }).catch(() => {});
					});
				break;
			}
		}

		// Forward event to active prompt() generator if listening
		if (this.promptEventCallback) {
			this.promptEventCallback(event);
		}
	}
}

/**
 * OpenCodeClient constructor options
 */
export interface OpenCodeClientOptions {
	cwd?: string;
	model?: string;
	env?: Record<string, string>;
}
