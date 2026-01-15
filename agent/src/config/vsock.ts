import { type ChildProcess, spawn } from "node:child_process";
import type { VsockConfig } from "./metadata.js";

let socatProcess: ChildProcess | null = null;

/**
 * Starts socat to forward vsock connections to a TCP port.
 *
 * socat listens on the vsock port and forwards to localhost:targetPort.
 * This allows the host to connect via vsock while the agent listens on TCP.
 *
 * @param vsockConfig - The vsock configuration from metadata
 * @param targetPort - The TCP port to forward to (agent's HTTP server port)
 * @returns Promise that resolves when socat is ready, or rejects on error
 */
export async function startVsockForwarder(
	vsockConfig: VsockConfig,
	targetPort: number,
): Promise<void> {
	const vsockPort = vsockConfig.port;
	const tcpPort = vsockConfig.target_port || targetPort;

	// Build socat command
	// VSOCK-LISTEN:<port> - Listen on vsock port, accept from any CID
	// fork - Handle multiple connections
	// reuseaddr - Allow quick restart
	// TCP:127.0.0.1:<port> - Forward to local TCP port
	const socatArgs = [
		`VSOCK-LISTEN:${vsockPort},reuseaddr,fork`,
		`TCP:127.0.0.1:${tcpPort}`,
	];

	console.log(`Starting vsock forwarder: socat ${socatArgs.join(" ")}`);

	return new Promise((resolve, reject) => {
		socatProcess = spawn("socat", socatArgs, {
			stdio: ["ignore", "pipe", "pipe"],
		});

		let started = false;
		let errorOutput = "";

		socatProcess.on("error", (err) => {
			if (!started) {
				reject(new Error(`Failed to start socat: ${err.message}`));
			} else {
				console.error(`socat error: ${err.message}`);
			}
		});

		socatProcess.stderr?.on("data", (data) => {
			const message = data.toString();
			errorOutput += message;
			// socat logs to stderr, but that's normal
			if (message.includes("E ")) {
				console.error(`socat: ${message.trim()}`);
			}
		});

		socatProcess.on("exit", (code, signal) => {
			if (!started) {
				reject(
					new Error(
						`socat exited prematurely with code ${code}, signal ${signal}: ${errorOutput}`,
					),
				);
			} else {
				console.log(`socat exited with code ${code}, signal ${signal}`);
				socatProcess = null;
			}
		});

		// socat doesn't have a "ready" signal, so we just wait briefly
		// and check if it's still running
		setTimeout(() => {
			if (socatProcess && !socatProcess.killed) {
				started = true;
				console.log(
					`Vsock forwarder started: vsock:${vsockPort} → tcp:127.0.0.1:${tcpPort}`,
				);
				resolve();
			}
		}, 100);
	});
}

/**
 * Stops the socat forwarder if running.
 */
export function stopVsockForwarder(): void {
	if (socatProcess && !socatProcess.killed) {
		console.log("Stopping vsock forwarder...");
		socatProcess.kill("SIGTERM");
		socatProcess = null;
	}
}

/**
 * Checks if socat is available on the system.
 */
export async function isSocatAvailable(): Promise<boolean> {
	return new Promise((resolve) => {
		const proc = spawn("which", ["socat"], { stdio: "ignore" });
		proc.on("exit", (code) => {
			resolve(code === 0);
		});
		proc.on("error", () => {
			resolve(false);
		});
	});
}
