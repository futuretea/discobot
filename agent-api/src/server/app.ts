import { Hono } from "hono";
import { logger } from "hono/logger";
import { streamSSE } from "hono/streaming";
import { ACPClient } from "../acp/client.js";
import type {
	ChatRequest,
	ChatStatusResponse,
	ClearSessionResponse,
	DiffFilesResponse,
	DiffResponse,
	ErrorResponse,
	GetMessagesResponse,
	HealthResponse,
	ListFilesResponse,
	ReadFileResponse,
	RootResponse,
	SingleFileDiffResponse,
	WriteFileRequest,
	WriteFileResponse,
} from "../api/types.js";
import { authMiddleware } from "../auth/middleware.js";
import {
	clearSession,
	getCompletionEvents,
	getCompletionState,
	getMessages,
	isCompletionRunning,
} from "../store/session.js";
import { tryStartCompletion } from "./completion.js";
import {
	getDiff,
	isFileError,
	listDirectory,
	readFile,
	writeFile,
} from "./files.js";

// Header name for credentials passed from server
const CREDENTIALS_HEADER = "X-Octobot-Credentials";

export interface AppOptions {
	agentCommand: string;
	agentArgs: string[];
	agentCwd: string;
	enableLogging?: boolean;
	/** Salted hash of shared secret (from OCTOBOT_SECRET env var) for auth enforcement */
	sharedSecretHash?: string;
}

export function createApp(options: AppOptions) {
	const app = new Hono();

	const acpClient = new ACPClient({
		command: options.agentCommand,
		args: options.agentArgs,
		cwd: options.agentCwd,
	});

	if (options.enableLogging) {
		app.use("*", logger());
	}

	if (options.sharedSecretHash) {
		app.use("*", authMiddleware(options.sharedSecretHash));
	}

	app.get("/", (c) => {
		return c.json<RootResponse>({ status: "ok", service: "agent" });
	});

	app.get("/health", (c) => {
		return c.json<HealthResponse>({
			healthy: true,
			connected: acpClient.isConnected,
		});
	});

	// GET /chat - Return messages (JSON) or stream events (SSE)
	// Content negotiation: Accept: text/event-stream returns SSE, otherwise JSON
	app.get("/chat", async (c) => {
		const accept = c.req.header("Accept") || "";

		// SSE mode: stream completion events for replay
		if (accept.includes("text/event-stream")) {
			// If no completion running, return 204 No Content
			if (!isCompletionRunning()) {
				return c.body(null, 204);
			}

			// Stream all events (past and future) until completion finishes
			return streamSSE(c, async (stream) => {
				// Send all events accumulated so far
				let lastEventIndex = 0;

				const sendNewEvents = async () => {
					const events = getCompletionEvents();
					while (lastEventIndex < events.length) {
						await stream.writeSSE({
							data: JSON.stringify(events[lastEventIndex]),
						});
						lastEventIndex++;
					}
				};

				// Send initial batch
				await sendNewEvents();

				// Poll for new events until completion finishes
				while (isCompletionRunning()) {
					await new Promise((resolve) => setTimeout(resolve, 50));
					await sendNewEvents();
				}

				// Send any final events
				await sendNewEvents();

				// Send [DONE] signal
				await stream.writeSSE({ data: "[DONE]" });
			});
		}

		// JSON mode: return all messages
		if (!acpClient.isConnected) {
			await acpClient.connect();
		}
		await acpClient.ensureSession();
		return c.json<GetMessagesResponse>({ messages: getMessages() });
	});

	// POST /chat - Start completion (runs in background, returns 202 Accepted)
	// Only one completion can run at a time - returns 409 Conflict if busy
	app.post("/chat", async (c) => {
		const body = await c.req.json<ChatRequest>();
		const credentialsHeader = c.req.header(CREDENTIALS_HEADER) || null;
		const result = tryStartCompletion(acpClient, body, credentialsHeader);
		return c.json(result.response, result.status);
	});

	// GET /chat/status - Get completion status
	app.get("/chat/status", (c) => {
		return c.json<ChatStatusResponse>(getCompletionState());
	});

	// DELETE /chat - Clear session and messages
	app.delete("/chat", async (c) => {
		await clearSession();
		return c.json<ClearSessionResponse>({ success: true });
	});

	// =========================================================================
	// File System Endpoints
	// =========================================================================

	// GET /files - List directory contents
	app.get("/files", async (c) => {
		const path = c.req.query("path") || ".";
		const hidden = c.req.query("hidden") === "true";

		const result = await listDirectory(path, {
			workspaceRoot: options.agentCwd,
			includeHidden: hidden,
		});

		if (isFileError(result)) {
			return c.json<ErrorResponse>({ error: result.error }, result.status);
		}
		return c.json<ListFilesResponse>(result);
	});

	// GET /files/read - Read file content
	app.get("/files/read", async (c) => {
		const path = c.req.query("path");
		if (!path) {
			return c.json<ErrorResponse>(
				{ error: "path query parameter required" },
				400,
			);
		}

		const result = await readFile(path, { workspaceRoot: options.agentCwd });

		if (isFileError(result)) {
			return c.json<ErrorResponse>({ error: result.error }, result.status);
		}
		return c.json<ReadFileResponse>(result);
	});

	// POST /files/write - Write file content
	app.post("/files/write", async (c) => {
		const body = await c.req.json<WriteFileRequest>();

		if (!body.path) {
			return c.json<ErrorResponse>({ error: "path is required" }, 400);
		}
		if (body.content === undefined) {
			return c.json<ErrorResponse>({ error: "content is required" }, 400);
		}

		const result = await writeFile(body.path, body.content, body.encoding, {
			workspaceRoot: options.agentCwd,
		});

		if (isFileError(result)) {
			return c.json<ErrorResponse>({ error: result.error }, result.status);
		}
		return c.json<WriteFileResponse>(result);
	});

	// GET /diff - Get session diff
	app.get("/diff", async (c) => {
		const path = c.req.query("path");
		const format = c.req.query("format") as "full" | "files" | undefined;

		const result = await getDiff(options.agentCwd, { path, format });

		if (isFileError(result)) {
			return c.json<ErrorResponse>({ error: result.error }, result.status);
		}

		// Type narrow based on what was returned
		if (path) {
			return c.json<SingleFileDiffResponse>(result as SingleFileDiffResponse);
		}
		if (format === "files") {
			return c.json<DiffFilesResponse>(result as DiffFilesResponse);
		}
		return c.json<DiffResponse>(result as DiffResponse);
	});

	return { app, acpClient };
}
