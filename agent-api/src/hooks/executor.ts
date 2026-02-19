/**
 * Hook Executor
 *
 * Executes hook scripts with timeout, output capture, and proper environment setup.
 * Output is saved to log files for persistence and LLM reference.
 */

import { type ChildProcess, spawn } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import type { Hook } from "./parser.js";

/**
 * Result of executing a hook
 */
export interface HookResult {
	/** Whether the hook succeeded (exit code 0) */
	success: boolean;
	/** Process exit code */
	exitCode: number;
	/** Combined stdout + stderr output */
	output: string;
	/** Hook that was executed */
	hook: Hook;
	/** Execution duration in milliseconds */
	durationMs: number;
}

/**
 * Options for hook execution
 */
export interface ExecuteHookOptions {
	/** Working directory */
	cwd: string;
	/** Additional environment variables */
	env?: Record<string, string>;
	/** Timeout in milliseconds (default: 30000) */
	timeout?: number;
	/** Changed files (for file hooks) - space-separated relative paths */
	changedFiles?: string[];
	/** Session ID */
	sessionId?: string;
	/** Path to save output log */
	outputPath?: string;
}

const DEFAULT_TIMEOUT = 15 * 60 * 1000; // 15 minutes

/**
 * Execute a hook script and capture its output.
 *
 * - Spawns the hook as a child process
 * - Sets environment variables (DISCOBOT_CHANGED_FILES, DISCOBOT_HOOK_TYPE, etc.)
 * - Captures combined stdout/stderr
 * - Enforces timeout
 * - Saves output to log file if outputPath is provided
 */
export async function executeHook(
	hook: Hook,
	opts: ExecuteHookOptions,
): Promise<HookResult> {
	const timeout = opts.timeout ?? DEFAULT_TIMEOUT;
	const startTime = Date.now();

	// Build environment
	const env: Record<string, string> = {
		...process.env,
		...opts.env,
		DISCOBOT_HOOK_TYPE: hook.type,
	};

	if (opts.sessionId) {
		env.DISCOBOT_SESSION_ID = opts.sessionId;
	}

	if (opts.cwd) {
		env.DISCOBOT_WORKSPACE = opts.cwd;
	}

	if (opts.changedFiles && opts.changedFiles.length > 0) {
		env.DISCOBOT_CHANGED_FILES = opts.changedFiles.join(" ");
	}

	let proc: ChildProcess;
	let killed = false;
	const outputChunks: string[] = [];

	try {
		proc = spawn(hook.path, [], {
			cwd: opts.cwd,
			stdio: ["pipe", "pipe", "pipe"],
			env,
		});
	} catch (err) {
		const output = `Failed to spawn hook: ${err instanceof Error ? err.message : String(err)}`;
		if (opts.outputPath) {
			await saveOutput(opts.outputPath, output);
		}
		return {
			success: false,
			exitCode: 127,
			output,
			hook,
			durationMs: Date.now() - startTime,
		};
	}

	// Collect output from both stdout and stderr
	proc.stdout?.on("data", (data: Buffer) => {
		outputChunks.push(data.toString());
	});

	proc.stderr?.on("data", (data: Buffer) => {
		outputChunks.push(data.toString());
	});

	// Timeout handler
	const timer = setTimeout(() => {
		killed = true;
		// Kill the process group if possible
		if (proc.pid) {
			try {
				process.kill(-proc.pid, "SIGKILL");
			} catch {
				proc.kill("SIGKILL");
			}
		} else {
			proc.kill("SIGKILL");
		}
	}, timeout);

	return new Promise<HookResult>((resolve) => {
		proc.on("exit", async (code) => {
			clearTimeout(timer);

			const exitCode = killed ? 124 : (code ?? 1); // 124 = timeout convention

			if (killed) {
				outputChunks.push(
					`\n[Hook timed out after ${timeout / 1000}s and was killed]\n`,
				);
			}

			const finalOutput = outputChunks.join("");

			if (opts.outputPath) {
				await saveOutput(opts.outputPath, finalOutput);
			}

			resolve({
				success: exitCode === 0,
				exitCode,
				output: finalOutput,
				hook,
				durationMs: Date.now() - startTime,
			});
		});

		proc.on("error", async (err) => {
			clearTimeout(timer);

			const output = `Hook execution error: ${err.message}`;
			if (opts.outputPath) {
				await saveOutput(opts.outputPath, output);
			}

			resolve({
				success: false,
				exitCode: 126,
				output,
				hook,
				durationMs: Date.now() - startTime,
			});
		});
	});
}

/**
 * Save hook output to a log file. Creates parent directories if needed.
 */
async function saveOutput(outputPath: string, output: string): Promise<void> {
	try {
		await mkdir(dirname(outputPath), { recursive: true });
		await writeFile(outputPath, output, "utf-8");
	} catch (err) {
		console.error(`Failed to save hook output to ${outputPath}:`, err);
	}
}

/**
 * Get the output log file path for a hook.
 */
export function getHookOutputPath(
	hooksDataDir: string,
	hookId: string,
): string {
	return join(hooksDataDir, "output", `${hookId}.log`);
}
