import { hasMetadata, loadConfig, METADATA_PATH } from "./config/metadata.js";
import { isSocatAvailable, startVsockForwarder } from "./config/vsock.js";
import { createApp } from "./server/app.js";

// Load configuration from VirtioFS metadata or environment variables
const config = loadConfig();

const { app } = createApp({
	agentCommand: config.agentCommand,
	agentArgs: config.agentArgs,
	agentCwd: config.agentCwd,
	enableLogging: true,
	sharedSecretHash: config.sharedSecretHash,
});

// Use Bun's native serve if available, otherwise fall back to Node
declare const Bun:
	| { serve: (options: { fetch: typeof app.fetch; port: number }) => void }
	| undefined;

async function startServer() {
	if (typeof Bun !== "undefined") {
		Bun.serve({
			fetch: app.fetch,
			port: config.port,
		});
	} else {
		// Node.js fallback
		const { serve } = await import("@hono/node-server");
		serve({
			fetch: app.fetch,
			port: config.port,
		});
	}
}

async function main() {
	console.log(`Starting agent service on port ${config.port}`);
	console.log(
		`Agent command: ${config.agentCommand} ${config.agentArgs.join(" ")}`,
	);
	console.log(`Agent cwd: ${config.agentCwd}`);
	console.log(
		`Auth enforcement: ${config.sharedSecretHash ? "enabled" : "disabled"}`,
	);

	if (hasMetadata()) {
		console.log(`VirtioFS metadata: ${METADATA_PATH}`);
		if (config.sessionId) {
			console.log(`Session ID: ${config.sessionId}`);
		}
	}

	// Start vsock forwarder if configured
	if (config.vsock) {
		const hasSocat = await isSocatAvailable();
		if (!hasSocat) {
			console.error(
				"ERROR: vsock forwarding configured but socat is not installed",
			);
			console.error("Install socat or remove vsock configuration");
			process.exit(1);
		}

		try {
			await startVsockForwarder(config.vsock, config.port);
			console.log(
				`Vsock forwarding: vsock:${config.vsock.port} → tcp:${config.port}`,
			);
		} catch (err) {
			console.error("Failed to start vsock forwarder:", err);
			process.exit(1);
		}
	}

	// Start the HTTP server
	await startServer();
	console.log(`Agent server listening on port ${config.port}`);
}

main().catch((err) => {
	console.error("Fatal error:", err);
	process.exit(1);
});
