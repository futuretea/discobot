/**
 * Unit tests for hook status store
 */

import assert from "node:assert";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, describe, it } from "node:test";
import type { HookResult } from "./executor.js";
import type { Hook } from "./parser.js";
import {
	addPendingHooks,
	getPendingHookIds,
	loadStatus,
	removePendingHook,
	saveStatus,
	updateHookStatus,
	updateLastEvaluatedAt,
} from "./status.js";

describe("hook status store", () => {
	let tempDir: string;

	beforeEach(async () => {
		tempDir = await mkdtemp(join(tmpdir(), "hook-status-test-"));
	});

	afterEach(async () => {
		await rm(tempDir, { recursive: true, force: true });
	});

	describe("loadStatus", () => {
		it("returns empty status when file does not exist", async () => {
			const status = await loadStatus(tempDir);
			assert.deepStrictEqual(status.hooks, {});
			assert.deepStrictEqual(status.pendingHooks, []);
			assert.strictEqual(status.lastEvaluatedAt, "");
		});

		it("loads saved status", async () => {
			await saveStatus(tempDir, {
				hooks: {
					"test-hook": {
						hookId: "test-hook",
						hookName: "Test Hook",
						type: "file",
						lastRunAt: "2024-01-01T00:00:00.000Z",
						lastResult: "success",
						lastExitCode: 0,
						outputPath: "/tmp/test.log",
						runCount: 5,
						failCount: 1,
						consecutiveFailures: 0,
					},
				},
				pendingHooks: [],
				lastEvaluatedAt: "2024-01-01T00:00:00.000Z",
			});

			const status = await loadStatus(tempDir);
			assert.strictEqual(Object.keys(status.hooks).length, 1);
			assert.strictEqual(status.hooks["test-hook"].runCount, 5);
			assert.strictEqual(status.hooks["test-hook"].failCount, 1);
		});
	});

	describe("updateHookStatus", () => {
		it("creates status for a new hook", async () => {
			const hook: Hook = {
				id: "my-hook",
				name: "My Hook",
				type: "file",
				path: "/tmp/my-hook.sh",
				runAs: "user",
				notifyLlm: true,
			};

			const result: HookResult = {
				success: true,
				exitCode: 0,
				output: "ok",
				hook,
				durationMs: 100,
			};

			await updateHookStatus(tempDir, result, "/tmp/output.log");

			const status = await loadStatus(tempDir);
			const hookStatus = status.hooks["my-hook"];
			assert.ok(hookStatus);
			assert.strictEqual(hookStatus.lastResult, "success");
			assert.strictEqual(hookStatus.runCount, 1);
			assert.strictEqual(hookStatus.failCount, 0);
			assert.strictEqual(hookStatus.consecutiveFailures, 0);
		});

		it("increments failure counts", async () => {
			const hook: Hook = {
				id: "fail-hook",
				name: "Fail Hook",
				type: "file",
				path: "/tmp/fail.sh",
				runAs: "user",
				notifyLlm: true,
			};

			const failResult: HookResult = {
				success: false,
				exitCode: 1,
				output: "error",
				hook,
				durationMs: 50,
			};

			await updateHookStatus(tempDir, failResult, "/tmp/output.log");
			await updateHookStatus(tempDir, failResult, "/tmp/output.log");

			const status = await loadStatus(tempDir);
			const hookStatus = status.hooks["fail-hook"];
			assert.strictEqual(hookStatus.runCount, 2);
			assert.strictEqual(hookStatus.failCount, 2);
			assert.strictEqual(hookStatus.consecutiveFailures, 2);
		});

		it("resets consecutive failures on success", async () => {
			const hook: Hook = {
				id: "reset-hook",
				name: "Reset Hook",
				type: "file",
				path: "/tmp/reset.sh",
				runAs: "user",
				notifyLlm: true,
			};

			// Fail twice
			await updateHookStatus(
				tempDir,
				{ success: false, exitCode: 1, output: "", hook, durationMs: 0 },
				"/tmp/out.log",
			);
			await updateHookStatus(
				tempDir,
				{ success: false, exitCode: 1, output: "", hook, durationMs: 0 },
				"/tmp/out.log",
			);

			// Then succeed
			await updateHookStatus(
				tempDir,
				{ success: true, exitCode: 0, output: "", hook, durationMs: 0 },
				"/tmp/out.log",
			);

			const status = await loadStatus(tempDir);
			const hookStatus = status.hooks["reset-hook"];
			assert.strictEqual(hookStatus.runCount, 3);
			assert.strictEqual(hookStatus.failCount, 2);
			assert.strictEqual(hookStatus.consecutiveFailures, 0);
		});
	});

	describe("pendingHooks", () => {
		it("addPendingHooks adds hook IDs as a set union", async () => {
			await addPendingHooks(tempDir, ["hook-a", "hook-b"]);

			let ids = await getPendingHookIds(tempDir);
			assert.deepStrictEqual(ids, ["hook-a", "hook-b"]);

			// Adding again with overlap should not duplicate
			await addPendingHooks(tempDir, ["hook-b", "hook-c"]);
			ids = await getPendingHookIds(tempDir);
			assert.deepStrictEqual(ids, ["hook-a", "hook-b", "hook-c"]);
		});

		it("removePendingHook removes a single ID", async () => {
			await addPendingHooks(tempDir, ["hook-a", "hook-b", "hook-c"]);
			await removePendingHook(tempDir, "hook-b");

			const ids = await getPendingHookIds(tempDir);
			assert.deepStrictEqual(ids, ["hook-a", "hook-c"]);
		});

		it("removePendingHook is a no-op for missing ID", async () => {
			await addPendingHooks(tempDir, ["hook-a"]);
			await removePendingHook(tempDir, "nonexistent");

			const ids = await getPendingHookIds(tempDir);
			assert.deepStrictEqual(ids, ["hook-a"]);
		});

		it("getPendingHookIds returns empty array when none pending", async () => {
			const ids = await getPendingHookIds(tempDir);
			assert.deepStrictEqual(ids, []);
		});

		it("pendingHooks defaults to empty array for old status files", async () => {
			// Simulate a status file written before pendingHooks was added
			const { writeFile, mkdir } = await import("node:fs/promises");
			await mkdir(tempDir, { recursive: true });
			await writeFile(
				join(tempDir, "status.json"),
				JSON.stringify({ hooks: {}, lastEvaluatedAt: "" }),
				"utf-8",
			);

			const status = await loadStatus(tempDir);
			assert.deepStrictEqual(status.pendingHooks, []);
		});
	});

	describe("updateLastEvaluatedAt", () => {
		it("sets the lastEvaluatedAt timestamp", async () => {
			await updateLastEvaluatedAt(tempDir);

			const status = await loadStatus(tempDir);
			assert.ok(status.lastEvaluatedAt);
			// Should be a valid ISO date
			assert.ok(!Number.isNaN(Date.parse(status.lastEvaluatedAt)));
		});
	});
});
