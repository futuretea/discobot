import type { ServerWebSocket } from "bun";
import type { McpServerConfig } from "@anthropic-ai/claude-agent-sdk";
import { createApp } from "./server/app.js";
import {
	buildTargetHeaders,
	createBunWebSocketHandler,
	type WebSocketData,
} from "./services/websocket-proxy.js";

// Load configuration from environment variables
const agentCwd = process.env.AGENT_CWD || process.cwd();
const port = Number(process.env.PORT) || 3002;
const sharedSecretHash = process.env.DISCOBOT_SECRET;
if (process.env.DISCOBOT_SECRET) {
	delete process.env.DISCOBOT_SECRET;
}

// app and getServicePort are initialized in main() after MCP servers are parsed.
// biome-ignore lint/style/noNonNullAssertion: assigned in main() before startServer() uses them
let app: ReturnType<typeof createApp>["app"] = null!;
// biome-ignore lint/style/noNonNullAssertion: assigned in main() before startServer() uses them
let getServicePort: ReturnType<typeof createApp>["getServicePort"] = null!;

// Use Bun's native serve if available, otherwise fall back to Node
declare const Bun:
	| {
			serve: (options: {
				fetch: (
					req: Request,
					server: { upgrade: (req: Request, options?: object) => boolean },
				) => Response | Promise<Response>;
				port: number;
				/** Disable idle timeout (0 = no timeout) */
				idleTimeout?: number;
				websocket: {
					open: (ws: ServerWebSocket<WebSocketData>) => void;
					message: (
						ws: ServerWebSocket<WebSocketData>,
						message: string | ArrayBuffer,
					) => void;
					close: (
						ws: ServerWebSocket<WebSocketData>,
						code: number,
						reason: string,
					) => void;
					/** Disable idle timeout for WebSocket connections */
					idleTimeout?: number;
				};
			}) => void;
	  }
	| undefined;

// MCPServer matches the Go MCPServer struct serialised by the server.
interface MCPServerConfig {
	id: string;
	name: string;
	type: string; // "stdio" or "http"
	command?: string;
	args?: string[];
	env?: string[]; // KEY=VALUE format
	url?: string;
	headers?: string[]; // KEY=VALUE or KEY: VALUE format
}

/**
 * Reads DISCOBOT_MCP_SERVERS from env and returns the SDK-format mcpServers
 * map to be passed directly to the Claude Agent SDK via Options.mcpServers.
 */
async function injectMCPSettings(): Promise<Record<string, McpServerConfig>> {
	const raw = process.env.DISCOBOT_MCP_SERVERS;
	if (!raw) return {};

	// Clean up env var — it contains sensitive server configs.
	delete process.env.DISCOBOT_MCP_SERVERS;

	let servers: MCPServerConfig[];
	try {
		servers = JSON.parse(raw) as MCPServerConfig[];
	} catch (err) {
		console.error("[mcp] Failed to parse DISCOBOT_MCP_SERVERS:", err);
		return {};
	}

	if (!Array.isArray(servers) || servers.length === 0) return {};

	const sdkMcpServers: Record<string, McpServerConfig> = {};

	for (const srv of servers) {
		const name = srv.name || srv.id;
		if (!name) continue;

		if (srv.type === "http" || srv.url) {
			// HTTP server
			const headerMap: Record<string, string> = {};
			for (const h of srv.headers ?? []) {
				// Accept "KEY=VALUE" and "KEY: VALUE" formats.
				const eqIdx = h.indexOf("=");
				const colonIdx = h.indexOf(": ");
				if (colonIdx !== -1 && (eqIdx === -1 || colonIdx < eqIdx)) {
					const key = h.slice(0, colonIdx).trim();
					const value = h.slice(colonIdx + 2).trim();
					if (key) headerMap[key] = value;
				} else if (eqIdx !== -1) {
					const key = h.slice(0, eqIdx);
					const value = h.slice(eqIdx + 1);
					if (key) headerMap[key] = value;
				}
			}
			sdkMcpServers[name] = {
				type: "http",
				url: srv.url ?? "",
				...(Object.keys(headerMap).length > 0 && { headers: headerMap }),
			};
		} else {
			// stdio server
			const envMap: Record<string, string> = {};
			for (const e of srv.env ?? []) {
				const idx = e.indexOf("=");
				if (idx !== -1) {
					const key = e.slice(0, idx);
					const value = e.slice(idx + 1);
					if (key) envMap[key] = value;
				}
			}
			sdkMcpServers[name] = {
				type: "stdio",
				command: srv.command ?? "",
				...(srv.args && srv.args.length > 0 && { args: srv.args }),
				...(Object.keys(envMap).length > 0 && { env: envMap }),
			};
		}
	}

	const count = Object.keys(sdkMcpServers).length;
	if (count === 0) return {};

	console.log(`[mcp] Loaded ${count} MCP server(s):`, Object.keys(sdkMcpServers).join(", "));
	return sdkMcpServers;
}

const SERVICE_HTTP_PATTERN = /^\/services\/([^/]+)\/http(\/.*)?$/;

async function startServer() {
	if (typeof Bun !== "undefined") {
		const wsHandler = createBunWebSocketHandler();

		Bun.serve({
			fetch: async (req, server) => {
				// Check if this is a WebSocket upgrade request for a service
				const upgradeHeader = req.headers.get("upgrade")?.toLowerCase();
				if (upgradeHeader === "websocket") {
					const url = new URL(req.url);
					const match = url.pathname.match(SERVICE_HTTP_PATTERN);

					if (match) {
						const serviceId = match[1];
						const forwardedPath =
							req.headers.get("x-forwarded-path") || match[2] || "/";

						// Get the service port
						const port = await getServicePort(serviceId);
						if (!port) {
							return new Response(
								JSON.stringify({
									error: "service_not_available",
									message: "Service not found or not running",
								}),
								{
									status: 502,
									headers: { "content-type": "application/json" },
								},
							);
						}

						// Build target WebSocket URL
						const targetUrl = `ws://localhost:${port}${forwardedPath}${url.search}`;

						// Build headers to forward to the target service
						const headers = buildTargetHeaders(req.headers, port);

						console.log(
							`[ws-proxy] Upgrading WebSocket: ${url.pathname} -> ${targetUrl}`,
						);

						// Upgrade the connection
						const upgraded = server.upgrade(req, {
							data: { targetUrl, serviceId, headers } satisfies WebSocketData,
						});

						if (upgraded) {
							// Return undefined to signal successful upgrade
							return undefined as unknown as Response;
						}

						return new Response("WebSocket upgrade failed", { status: 500 });
					}
				}

				// Fall through to Hono for regular HTTP requests
				return app.fetch(req);
			},
			port: port,
			// Disable idle timeout for HTTP connections (0 = no timeout)
			// This is important for long-running SSE streams and proxied connections
			idleTimeout: 0,
			websocket: {
				...wsHandler,
				// Disable idle timeout for WebSocket connections
				idleTimeout: 0,
			},
		});
	} else {
		// Node.js fallback - no WebSocket support for now
		const { serve } = await import("@hono/node-server");
		const server = serve(
			{
				fetch: app.fetch,
				port: port,
				serverOptions: {
					// Disable request timeout (important for SSE and long-running connections)
					requestTimeout: 0,
					// Disable keep-alive timeout
					keepAliveTimeout: 0,
					// Disable headers timeout
					headersTimeout: 0,
				},
			},
			() => {
				console.log(
					"[warn] Running in Node.js mode - WebSocket proxy not supported",
				);
			},
		);
		// Also disable timeout on the server itself (cast to access timeout property)
		(server as { timeout?: number }).timeout = 0;
	}
}

async function main() {
	console.log(`Starting agent service on port ${port}`);
	console.log(`Agent cwd: ${agentCwd}`);
	console.log(`Auth enforcement: ${sharedSecretHash ? "enabled" : "disabled"}`);

	// Parse MCP servers from DISCOBOT_MCP_SERVERS env var.
	// Returns SDK-format mcpServers to be passed directly to the Claude Agent SDK.
	const mcpServers = await injectMCPSettings();

	// Create the app now that mcpServers are known.
	({ app, getServicePort } = createApp({
		agentCwd,
		enableLogging: true,
		sharedSecretHash,
		mcpServers,
	}));

	// Start the HTTP server
	await startServer();
	console.log(`Agent server listening on port ${port}`);
}

main().catch((err) => {
	console.error("Fatal error:", err);
	process.exit(1);
});
