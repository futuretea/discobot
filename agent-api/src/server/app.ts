import { exec } from "node:child_process";
import os from "node:os";
import { promisify } from "node:util";
import type { UIMessage } from "ai";
import { type Context, Hono } from "hono";
import { logger } from "hono/logger";
import { streamSSE } from "hono/streaming";
import type { Agent } from "../agent/interface.js";
import type {
	ChatRequest,
	ChatStatusResponse,
	CommitsErrorResponse,
	CommitsResponse,
	DeleteFileRequest,
	DeleteFileResponse,
	DiffFilesResponse,
	DiffResponse,
	ErrorResponse,
	GetMessagesResponse,
	HealthResponse,
	ListFilesResponse,
	ListServicesResponse,
	ModelsResponse,
	ReadFileResponse,
	RenameFileRequest,
	RenameFileResponse,
	RootResponse,
	ServiceAlreadyRunningResponse,
	ServiceIsPassiveResponse,
	ServiceNoPortResponse,
	ServiceNotFoundResponse,
	ServiceNotRunningResponse,
	ServiceOutputEvent,
	SingleFileDiffResponse,
	StartServiceResponse,
	StopServiceResponse,
	UserResponse,
	WriteFileRequest,
	WriteFileResponse,
} from "../api/types.js";
import { authMiddleware } from "../auth/middleware.js";
import { checkCredentialsChanged } from "../credentials/credentials.js";
import { HookManager } from "../hooks/manager.js";
import { questionManager } from "../question-manager.js";
import {
	getManagedService,
	getService,
	getServiceOutput,
	getServices,
	startService,
	stopService,
} from "../services/manager.js";
import { proxyHttpRequest } from "../services/proxy.js";
import {
	aggregateDeltas,
	getCompletionEvents,
	getCompletionState,
	isCompletionRunning,
} from "../store/session.js";
import { createAgent, isValidAgentType } from "./agents.js";
import { getCommitPatches, isCommitsError } from "./commits.js";
import {
	resetHookState,
	tryCancelCompletion,
	tryStartCompletion,
} from "./completion.js";
import {
	deleteFile,
	getDiff,
	isFileError,
	listDirectory,
	readFile,
	renameFile,
	searchFiles,
	writeFile,
} from "./files.js";

// Header names for credentials and git config passed from server
const CREDENTIALS_HEADER = "X-Discobot-Credentials";
const GIT_USER_NAME_HEADER = "X-Discobot-Git-User-Name";
const GIT_USER_EMAIL_HEADER = "X-Discobot-Git-User-Email";

const execAsync = promisify(exec);

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

// Debug logging — enabled by default, disable with DEBUG=false
const DEBUG = process.env.DEBUG !== "false";

function debug(msg: string, data?: Record<string, unknown>) {
	if (!DEBUG) return;
	const ts = new Date().toISOString();
	if (data) {
		console.log(`[debug ${ts}] ${msg}`, JSON.stringify(data));
	} else {
		console.log(`[debug ${ts}] ${msg}`);
	}
}

export interface AppOptions {
	agentCwd: string;
	enableLogging?: boolean;
	/** Salted hash of shared secret (from DISCOBOT_SECRET env var) for auth enforcement */
	sharedSecretHash?: string;
}

export function createApp(options: AppOptions) {
	const app = new Hono();

	// Lazy agent registry: agents are created on first use per type
	const agents = new Map<string, Agent>();

	function getAgent(agentType: string): Agent {
		let agent = agents.get(agentType);
		if (!agent) {
			agent = createAgent(agentType, {
				cwd: options.agentCwd,
				model: process.env.AGENT_MODEL,
				env: process.env as Record<string, string>,
			});
			agents.set(agentType, agent);
		}
		return agent;
	}

	// Initialize hook manager for file and pre-commit hooks (opt-in via env var).
	// The Go agent binary sets DISCOBOT_HOOKS_ENABLED=true when running in containers.
	// When running locally (e.g. local sandbox provider), hooks are disabled by default.
	let hookManager: HookManager | null = null;
	if (process.env.DISCOBOT_HOOKS_ENABLED === "true") {
		const sessionId = process.env.SESSION_ID || "default";
		hookManager = new HookManager(options.agentCwd, sessionId);
		hookManager.init().catch((err) => {
			console.error("[hooks] Failed to initialize hook manager:", err);
		});
	}

	if (options.enableLogging) {
		app.use("*", logger());
	}

	if (options.sharedSecretHash) {
		app.use("*", authMiddleware(options.sharedSecretHash));
	}

	// Middleware for all session-scoped agent routes: apply credential and git
	// user config from request headers before the route handler runs.
	app.use("/session/:id/:agent/*", async (c, next) => {
		const agentType = c.req.param("agent");
		if (!isValidAgentType(agentType)) {
			return next();
		}

		const agent = getAgent(agentType);
		const sessionId = c.req.param("id");

		const credentialsHeader = c.req.header(CREDENTIALS_HEADER) ?? null;
		const {
			changed,
			env: credentialEnv,
			credentials: rawCredentials,
		} = checkCredentialsChanged(credentialsHeader);
		if (changed) {
			await agent.updateEnvironment(sessionId, credentialEnv, rawCredentials);
		}

		const gitUserName = c.req.header(GIT_USER_NAME_HEADER) ?? null;
		const gitUserEmail = c.req.header(GIT_USER_EMAIL_HEADER) ?? null;
		if (gitUserName || gitUserEmail) {
			await configureGitUser(gitUserName, gitUserEmail);
		}

		return next();
	});

	// =========================================================================
	// Global (non-session) routes — not prefixed with /session/:id
	// =========================================================================

	app.get("/", (c) => {
		return c.json<RootResponse>({ status: "ok", service: "agent" });
	});

	app.get("/health", (c) => {
		return c.json<HealthResponse>({
			healthy: true,
			connected: agents.size > 0,
		});
	});

	// GET /user - Return current user info for terminal sessions
	app.get("/user", (c) => {
		const userInfo = os.userInfo();
		return c.json<UserResponse>({
			username: userInfo.username,
			uid: userInfo.uid,
			gid: userInfo.gid,
		});
	});

	// =========================================================================
	// Agent helper
	// =========================================================================

	// Helper to resolve agent from /:agent path parameter, returning 404 on unknown type
	const resolveAgent = (c: Context): Agent | null => {
		const agentType = c.req.param("agent");
		if (!isValidAgentType(agentType)) {
			return null;
		}
		return getAgent(agentType);
	};

	// =========================================================================
	// Session-scoped routes — all prefixed with /session/:id
	// =========================================================================

	// GET /session/:id/:agent/models - List available models
	app.get("/session/:id/:agent/models", async (c) => {
		const agent = resolveAgent(c);
		if (!agent) {
			return c.json<ErrorResponse>(
				{ error: `Unknown agent type: ${c.req.param("agent")}` },
				404,
			);
		}

		const sessionId = c.req.param("id");

		try {
			const models = await agent.listModels(sessionId);
			return c.json<ModelsResponse>({ models });
		} catch (error) {
			console.error("Failed to list models:", error);
			return c.json<ErrorResponse>(
				{
					error: `Failed to list models: ${error instanceof Error ? error.message : String(error)}`,
				},
				500,
			);
		}
	});

	// GET /session/:id/:agent/chat - Return messages (JSON) or stream events (SSE)
	app.get("/session/:id/:agent/chat", async (c) => {
		const agent = resolveAgent(c);
		if (!agent) {
			return c.json<ErrorResponse>(
				{ error: `Unknown agent type: ${c.req.param("agent")}` },
				404,
			);
		}

		const sessionId = c.req.param("id");
		const accept = c.req.header("Accept") || "";
		debug("GET /chat", { accept: accept.substring(0, 50) });

		// SSE mode: stream completion events for replay
		if (accept.includes("text/event-stream")) {
			if (!isCompletionRunning()) {
				debug("GET /chat SSE → 204 No Content (no completion running)");
				return c.body(null, 204);
			}

			debug("GET /chat SSE → streaming completion events");
			return streamSSE(c, async (stream) => {
				const initialEvents = getCompletionEvents();
				const aggregatedInitial = aggregateDeltas(initialEvents);

				for (const event of aggregatedInitial) {
					await stream.writeSSE({ data: JSON.stringify(event) });
				}

				let lastEventIndex = initialEvents.length;

				const sendNewEvents = async () => {
					const events = getCompletionEvents();
					while (lastEventIndex < events.length) {
						await stream.writeSSE({
							data: JSON.stringify(events[lastEventIndex]),
						});
						lastEventIndex++;
					}
				};

				while (isCompletionRunning()) {
					await new Promise((resolve) => setTimeout(resolve, 50));
					await sendNewEvents();
				}

				await sendNewEvents();
				await stream.writeSSE({ data: "[DONE]" });
			});
		}

		// JSON mode: return all messages
		let messages: UIMessage[];
		try {
			messages = await agent.getMessages(sessionId);
		} catch (error) {
			debug("GET /chat → 500 getMessages failed", { error: String(error) });
			console.error(`Failed to load messages for ${sessionId}:`, error);
			return c.json<ErrorResponse>(
				{
					error: `Failed to load messages: ${error instanceof Error ? error.message : String(error)}`,
				},
				500,
			);
		}

		// Strip trailing partial assistant message during active completion to
		// prevent the Vercel AI SDK from using it as a streaming base and
		// producing duplicate/garbled output.
		if (
			isCompletionRunning() &&
			messages.length > 0 &&
			messages[messages.length - 1].role === "assistant"
		) {
			messages = messages.slice(0, -1);
		}

		debug("GET /chat → 200 JSON", {
			messageCount: messages.length,
			completionRunning: isCompletionRunning(),
		});
		debug(`GET /chat → response messages: ${JSON.stringify(messages)}`);
		return c.json<GetMessagesResponse>({ messages });
	});

	// POST /session/:id/:agent/chat - Start a completion
	app.post("/session/:id/:agent/chat", async (c) => {
		const agent = resolveAgent(c);
		if (!agent) {
			return c.json<ErrorResponse>(
				{ error: `Unknown agent type: ${c.req.param("agent")}` },
				404,
			);
		}

		const sessionId = c.req.param("id");
		const body = await c.req.json<ChatRequest>();
		debug("POST /chat", {
			model: body.model,
			messageCount: body.messages?.length,
			hasCredentials: !!c.req.header(CREDENTIALS_HEADER),
		});

		resetHookState();
		const result = tryStartCompletion(
			agent,
			body,
			body.model,
			body.reasoning,
			body.mode,
			sessionId,
			hookManager,
		);

		debug(`POST /chat → ${result.status}`, { response: result.response });
		return c.json(result.response, result.status);
	});

	// GET /session/:id/:agent/chat/status - Get completion status
	app.get("/session/:id/:agent/chat/status", (c) => {
		if (!isValidAgentType(c.req.param("agent"))) {
			return c.json<ErrorResponse>(
				{ error: `Unknown agent type: ${c.req.param("agent")}` },
				404,
			);
		}
		return c.json<ChatStatusResponse>(getCompletionState());
	});

	// POST /session/:id/:agent/chat/cancel - Cancel in-progress completion
	app.post("/session/:id/:agent/chat/cancel", (c) => {
		if (!isValidAgentType(c.req.param("agent"))) {
			return c.json<ErrorResponse>(
				{ error: `Unknown agent type: ${c.req.param("agent")}` },
				404,
			);
		}
		const result = tryCancelCompletion();
		return c.json(result.response, result.status);
	});

	// GET /session/:id/:agent/chat/question - Return the current pending AskUserQuestion
	app.get("/session/:id/:agent/chat/question", (c) => {
		if (!isValidAgentType(c.req.param("agent"))) {
			return c.json<ErrorResponse>(
				{ error: `Unknown agent type: ${c.req.param("agent")}` },
				404,
			);
		}
		const toolUseID = c.req.query("toolUseID");
		const pending = questionManager.getPendingQuestion();
		if (toolUseID) {
			if (pending && pending.toolUseID === toolUseID) {
				return c.json({ status: "pending", question: pending });
			}
			return c.json({ status: "answered", question: null });
		}
		return c.json({ question: pending });
	});

	// POST /session/:id/:agent/chat/answer - Submit answers to a pending AskUserQuestion
	app.post("/session/:id/:agent/chat/answer", async (c) => {
		if (!isValidAgentType(c.req.param("agent"))) {
			return c.json<ErrorResponse>(
				{ error: `Unknown agent type: ${c.req.param("agent")}` },
				404,
			);
		}
		const body = await c.req.json<{
			toolUseID: string;
			answers: Record<string, string>;
		}>();
		const { toolUseID, answers } = body;
		if (!toolUseID || !answers) {
			return c.json({ error: "toolUseID and answers are required" }, 400);
		}
		const success = questionManager.submitAnswer(toolUseID, answers);
		if (!success) {
			return c.json({ error: "No pending question for this toolUseID" }, 404);
		}
		return c.json({ success: true });
	});

	// =========================================================================
	// Hook routes
	// =========================================================================

	// GET /hooks/status - Get hook evaluation status
	app.get("/hooks/status", async (c) => {
		if (!hookManager) {
			return c.json({ hooks: {}, pendingHooks: [], lastEvaluatedAt: "" });
		}
		const status = await hookManager.getStatus();
		return c.json(status);
	});

	// GET /hooks/:hookId/output - Get hook output log
	app.get("/hooks/:hookId/output", async (c) => {
		if (!hookManager) {
			return c.json<ErrorResponse>({ error: "Hooks not enabled" }, 404);
		}
		const hookId = c.req.param("hookId");
		const output = await hookManager.getHookOutput(hookId);
		if (output === null) {
			return c.json({ output: "" });
		}
		return c.json({ output });
	});

	// POST /hooks/:hookId/rerun - Manually rerun a hook
	app.post("/hooks/:hookId/rerun", async (c) => {
		if (!hookManager) {
			return c.json<ErrorResponse>({ error: "Hooks not enabled" }, 404);
		}
		const hookId = c.req.param("hookId");
		const result = await hookManager.rerunHook(hookId);
		if (!result) {
			return c.json<ErrorResponse>({ error: "Hook not found" }, 404);
		}
		return c.json({ success: result.success, exitCode: result.exitCode });
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

	// GET /files/search - Fuzzy search files in workspace
	app.get("/files/search", async (c) => {
		const query = c.req.query("q") ?? "";
		const limit = Math.min(Number(c.req.query("limit") ?? "50"), 200);

		const result = await searchFiles(query, {
			workspaceRoot: options.agentCwd,
			limit,
		});

		if (isFileError(result)) {
			return c.json<ErrorResponse>({ error: result.error }, result.status);
		}
		return c.json(result);
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

	// POST /files/delete - Delete a file or directory
	app.post("/files/delete", async (c) => {
		const body = await c.req.json<DeleteFileRequest>();

		if (!body.path) {
			return c.json<ErrorResponse>({ error: "path is required" }, 400);
		}

		const result = await deleteFile(body.path, {
			workspaceRoot: options.agentCwd,
		});

		if (isFileError(result)) {
			return c.json<ErrorResponse>({ error: result.error }, result.status);
		}
		return c.json<DeleteFileResponse>(result);
	});

	// POST /files/rename - Rename/move a file or directory
	app.post("/files/rename", async (c) => {
		const body = await c.req.json<RenameFileRequest>();

		if (!body.oldPath) {
			return c.json<ErrorResponse>({ error: "oldPath is required" }, 400);
		}
		if (!body.newPath) {
			return c.json<ErrorResponse>({ error: "newPath is required" }, 400);
		}

		const result = await renameFile(body.oldPath, body.newPath, {
			workspaceRoot: options.agentCwd,
		});

		if (isFileError(result)) {
			return c.json<ErrorResponse>({ error: result.error }, result.status);
		}
		return c.json<RenameFileResponse>(result);
	});

	// GET /diff - Get workspace diff
	app.get("/diff", async (c) => {
		const path = c.req.query("path");
		const format = c.req.query("format") as "full" | "files" | undefined;

		const result = await getDiff(options.agentCwd, {
			path,
			format,
		});

		if (isFileError(result)) {
			return c.json<ErrorResponse>({ error: result.error }, result.status);
		}

		if (path) {
			return c.json<SingleFileDiffResponse>(result as SingleFileDiffResponse);
		}
		if (format === "files") {
			return c.json<DiffFilesResponse>(result as DiffFilesResponse);
		}
		return c.json<DiffResponse>(result as DiffResponse);
	});

	// =========================================================================
	// Git Commits Endpoint
	// =========================================================================

	app.get("/commits", async (c) => {
		const parent = c.req.query("parent");
		if (!parent) {
			return c.json<CommitsErrorResponse>(
				{
					error: "invalid_parent",
					message: "parent query parameter is required",
				},
				400,
			);
		}

		const result = await getCommitPatches(options.agentCwd, parent);

		if (isCommitsError(result)) {
			const statusMap: Record<CommitsErrorResponse["error"], number> = {
				invalid_parent: 400,
				not_git_repo: 400,
				parent_mismatch: 409,
				no_commits: 404,
			};
			return c.json<CommitsErrorResponse>(
				result,
				statusMap[result.error] as 400 | 404 | 409,
			);
		}

		return c.json<CommitsResponse>(result);
	});

	// =========================================================================
	// Service Management Endpoints
	// =========================================================================

	// GET /services - List all services with status
	app.get("/services", async (c) => {
		const services = await getServices(options.agentCwd);
		return c.json<ListServicesResponse>({ services });
	});

	// POST /services/:serviceId/start - Start a service
	app.post("/services/:serviceId/start", async (c) => {
		const serviceId = c.req.param("serviceId");

		const service = await getService(options.agentCwd, serviceId);
		if (service?.passive) {
			return c.json<ServiceIsPassiveResponse>(
				{
					error: "service_is_passive",
					serviceId,
					message:
						"Passive services are externally managed and cannot be started",
				},
				400,
			);
		}

		const result = await startService(options.agentCwd, serviceId);

		if (!result.ok) {
			if (result.status === 404) {
				return c.json<ServiceNotFoundResponse>(
					result.response as ServiceNotFoundResponse,
					404,
				);
			}
			return c.json<ServiceAlreadyRunningResponse>(
				result.response as ServiceAlreadyRunningResponse,
				409,
			);
		}

		return c.json<StartServiceResponse>(result.response, 202);
	});

	// POST /services/:serviceId/stop - Stop a service
	app.post("/services/:serviceId/stop", async (c) => {
		const serviceId = c.req.param("serviceId");

		const service = await getService(options.agentCwd, serviceId);
		if (service?.passive) {
			return c.json<ServiceIsPassiveResponse>(
				{
					error: "service_is_passive",
					serviceId,
					message:
						"Passive services are externally managed and cannot be stopped",
				},
				400,
			);
		}

		const result = await stopService(serviceId);

		if (!result.ok) {
			if (result.status === 404) {
				return c.json<ServiceNotFoundResponse>(
					result.response as ServiceNotFoundResponse,
					404,
				);
			}
			return c.json<ServiceNotRunningResponse>(
				result.response as ServiceNotRunningResponse,
				400,
			);
		}

		return c.json<StopServiceResponse>(result.response, 200);
	});

	// GET /services/:serviceId/output - Stream service output via SSE
	app.get("/services/:serviceId/output", async (c) => {
		const serviceId = c.req.param("serviceId");

		const service = await getService(options.agentCwd, serviceId);
		if (service?.passive) {
			return c.json<ServiceIsPassiveResponse>(
				{
					error: "service_is_passive",
					serviceId,
					message:
						"Passive services are externally managed and have no output logs",
				},
				400,
			);
		}

		const managed = getManagedService(serviceId);

		return streamSSE(c, async (stream) => {
			// Send buffered events from file first (replay)
			const storedEvents = await getServiceOutput(serviceId);
			for (const event of storedEvents) {
				await stream.writeSSE({ data: JSON.stringify(event) });
			}

			// If no running service, send done and close
			if (!managed) {
				await stream.writeSSE({ data: "[DONE]" });
				return;
			}

			// If already stopped, send done and close
			if (managed.service.status === "stopped") {
				await stream.writeSSE({ data: "[DONE]" });
				return;
			}

			// Stream live events
			const onOutput = async (event: ServiceOutputEvent) => {
				try {
					await stream.writeSSE({ data: JSON.stringify(event) });
				} catch {
					// Stream may be closed
				}
			};

			const onClose = async () => {
				try {
					await stream.writeSSE({ data: "[DONE]" });
				} catch {
					// Stream may be closed
				}
			};

			managed.eventEmitter.on("output", onOutput);
			managed.eventEmitter.once("close", onClose);

			// Wait for close event or client disconnect
			await new Promise<void>((resolve) => {
				const cleanup = () => {
					managed.eventEmitter.off("output", onOutput);
					managed.eventEmitter.off("close", onClose);
					resolve();
				};

				managed.eventEmitter.once("close", cleanup);

				// Handle client disconnect
				c.req.raw.signal.addEventListener("abort", cleanup);
			});
		});
	});

	// ALL /services/:serviceId/http/* - HTTP reverse proxy to service port
	app.all("/services/:serviceId/http/*", async (c) => {
		const serviceId = c.req.param("serviceId");
		const service = await getService(options.agentCwd, serviceId);

		if (!service) {
			return c.json<ServiceNotFoundResponse>(
				{ error: "service_not_found", serviceId },
				404,
			);
		}

		const port = service.http || service.https;
		if (!port) {
			return c.json<ServiceNoPortResponse>(
				{ error: "service_no_port", serviceId },
				400,
			);
		}

		// For non-passive services that aren't running, auto-start them
		if (!service.passive && service.status !== "running") {
			const startResult = await startService(options.agentCwd, serviceId);

			if (!startResult.ok) {
				if (startResult.status !== 409) {
					return c.json(startResult.response, startResult.status);
				}
			}
		}

		return proxyHttpRequest(c, port);
	});

	/**
	 * Get the HTTP port for a service, auto-starting if needed.
	 */
	async function getServicePort(serviceId: string): Promise<number | null> {
		const service = await getService(options.agentCwd, serviceId);
		if (!service) return null;

		const port = service.http || service.https;
		if (!port) return null;

		if (service.passive) return port;

		if (service.status !== "running") {
			const startResult = await startService(options.agentCwd, serviceId);
			if (!startResult.ok && startResult.status !== 409) {
				return null;
			}
		}

		return port;
	}

	return { app, getAgent, getServicePort };
}
