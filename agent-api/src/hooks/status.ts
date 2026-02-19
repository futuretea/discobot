/**
 * Hook Status Store
 *
 * Persists hook run status to ~/.discobot/{sessionId}/hooks/status.json.
 * Survives container restarts via the overlay filesystem.
 * Uses atomic writes (write-to-temp + rename) for safety.
 */

import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import type { HookResult } from "./executor.js";

/**
 * Status of a single hook's runs
 */
export interface HookRunStatus {
	hookId: string;
	hookName: string;
	type: "session" | "file" | "pre-commit";
	lastRunAt: string;
	lastResult: "success" | "failure" | "running";
	lastExitCode: number;
	/** Path to the output log file */
	outputPath: string;
	runCount: number;
	failCount: number;
	consecutiveFailures: number;
}

/**
 * Top-level status file schema
 */
export interface HookStatusFile {
	hooks: Record<string, HookRunStatus>;
	/** Hook IDs that need to be run (matched to changed files but not yet passed) */
	pendingHooks: string[];
	lastEvaluatedAt: string;
}

/**
 * Get the hooks data directory for a session.
 */
export function getHooksDataDir(sessionId: string): string {
	const home = process.env.HOME || "/home/discobot";
	return join(home, ".discobot", sessionId, "hooks");
}

/**
 * Get the path to the status file.
 */
function getStatusFilePath(hooksDataDir: string): string {
	return join(hooksDataDir, "status.json");
}

/**
 * Get the path to the last-eval marker file.
 */
export function getLastEvalMarkerPath(hooksDataDir: string): string {
	return join(hooksDataDir, ".last-eval");
}

/**
 * Load the hook status file. Returns empty status if file doesn't exist.
 */
export async function loadStatus(
	hooksDataDir: string,
): Promise<HookStatusFile> {
	const filePath = getStatusFilePath(hooksDataDir);

	try {
		const content = await readFile(filePath, "utf-8");
		const status = JSON.parse(content) as HookStatusFile;
		// Handle files written before pendingHooks was added
		if (!status.pendingHooks) status.pendingHooks = [];
		return status;
	} catch {
		return {
			hooks: {},
			pendingHooks: [],
			lastEvaluatedAt: "",
		};
	}
}

/**
 * Save the hook status file atomically (write-to-temp + rename).
 */
export async function saveStatus(
	hooksDataDir: string,
	status: HookStatusFile,
): Promise<void> {
	const filePath = getStatusFilePath(hooksDataDir);
	const tmpPath = `${filePath}.tmp.${Date.now()}`;

	try {
		await mkdir(dirname(filePath), { recursive: true });
		await writeFile(tmpPath, JSON.stringify(status, null, "\t"), "utf-8");
		await rename(tmpPath, filePath);
	} catch (err) {
		console.error(`Failed to save hook status to ${filePath}:`, err);
		// Try to clean up temp file
		try {
			const { unlink } = await import("node:fs/promises");
			await unlink(tmpPath);
		} catch {
			// ignore cleanup failure
		}
	}
}

/**
 * Mark a hook as currently running in the status file.
 */
export async function setHookRunning(
	hooksDataDir: string,
	hook: { id: string; name: string; type: string },
): Promise<void> {
	const status = await loadStatus(hooksDataDir);

	const existing = status.hooks[hook.id];

	status.hooks[hook.id] = {
		hookId: hook.id,
		hookName: hook.name,
		type: hook.type as HookRunStatus["type"],
		lastRunAt: new Date().toISOString(),
		lastResult: "running",
		lastExitCode: existing?.lastExitCode ?? 0,
		outputPath: existing?.outputPath ?? "",
		runCount: existing?.runCount ?? 0,
		failCount: existing?.failCount ?? 0,
		consecutiveFailures: existing?.consecutiveFailures ?? 0,
	};

	await saveStatus(hooksDataDir, status);
}

/**
 * Update status for a single hook after execution.
 */
export async function updateHookStatus(
	hooksDataDir: string,
	result: HookResult,
	outputPath: string,
): Promise<void> {
	const status = await loadStatus(hooksDataDir);

	const existing = status.hooks[result.hook.id];
	const runCount = (existing?.runCount ?? 0) + 1;
	const failCount = (existing?.failCount ?? 0) + (result.success ? 0 : 1);
	const consecutiveFailures = result.success
		? 0
		: (existing?.consecutiveFailures ?? 0) + 1;

	status.hooks[result.hook.id] = {
		hookId: result.hook.id,
		hookName: result.hook.name,
		type: result.hook.type,
		lastRunAt: new Date().toISOString(),
		lastResult: result.success ? "success" : "failure",
		lastExitCode: result.exitCode,
		outputPath,
		runCount,
		failCount,
		consecutiveFailures,
	};

	await saveStatus(hooksDataDir, status);
}

/**
 * Update the lastEvaluatedAt timestamp in the status file.
 */
export async function updateLastEvaluatedAt(
	hooksDataDir: string,
): Promise<void> {
	const status = await loadStatus(hooksDataDir);
	status.lastEvaluatedAt = new Date().toISOString();
	await saveStatus(hooksDataDir, status);
}

/**
 * Add hook IDs to the pending set (set-union, no duplicates).
 */
export async function addPendingHooks(
	hooksDataDir: string,
	hookIds: string[],
): Promise<void> {
	const status = await loadStatus(hooksDataDir);
	const pending = new Set(status.pendingHooks);
	for (const id of hookIds) {
		pending.add(id);
	}
	status.pendingHooks = Array.from(pending);
	await saveStatus(hooksDataDir, status);
}

/**
 * Remove a single hook ID from the pending set.
 */
export async function removePendingHook(
	hooksDataDir: string,
	hookId: string,
): Promise<void> {
	const status = await loadStatus(hooksDataDir);
	status.pendingHooks = status.pendingHooks.filter((id) => id !== hookId);
	await saveStatus(hooksDataDir, status);
}

/**
 * Get the list of pending hook IDs.
 */
export async function getPendingHookIds(
	hooksDataDir: string,
): Promise<string[]> {
	const status = await loadStatus(hooksDataDir);
	return status.pendingHooks;
}
