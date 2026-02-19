/**
 * Unit tests for hook executor
 */

import assert from "node:assert";
import { chmod, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, beforeEach, describe, it } from "node:test";
import { executeHook, getHookOutputPath } from "./executor.js";
import type { Hook } from "./parser.js";

describe("executeHook", () => {
	let tempDir: string;

	beforeEach(async () => {
		tempDir = await mkdtemp(join(tmpdir(), "hook-executor-test-"));
	});

	afterEach(async () => {
		await rm(tempDir, { recursive: true, force: true });
	});

	function makeHook(overrides: Partial<Hook> = {}): Hook {
		return {
			id: "test-hook",
			name: "Test Hook",
			type: "file",
			path: join(tempDir, "test-hook.sh"),
			runAs: "user",
			notifyLlm: true,
			...overrides,
		};
	}

	it("executes a successful hook", async () => {
		const scriptPath = join(tempDir, "success.sh");
		await writeFile(scriptPath, '#!/bin/bash\necho "all good"', "utf-8");
		await chmod(scriptPath, 0o755);

		const hook = makeHook({ id: "success", path: scriptPath });
		const result = await executeHook(hook, { cwd: tempDir });

		assert.strictEqual(result.success, true);
		assert.strictEqual(result.exitCode, 0);
		assert.ok(result.output.includes("all good"));
		assert.ok(result.durationMs >= 0);
		assert.strictEqual(result.hook.id, "success");
	});

	it("captures failure exit code", async () => {
		const scriptPath = join(tempDir, "fail.sh");
		await writeFile(
			scriptPath,
			'#!/bin/bash\necho "error" >&2\nexit 1',
			"utf-8",
		);
		await chmod(scriptPath, 0o755);

		const hook = makeHook({ id: "fail", path: scriptPath });
		const result = await executeHook(hook, { cwd: tempDir });

		assert.strictEqual(result.success, false);
		assert.strictEqual(result.exitCode, 1);
		assert.ok(result.output.includes("error"));
	});

	it("enforces timeout", async () => {
		const scriptPath = join(tempDir, "slow.sh");
		await writeFile(scriptPath, "#!/bin/bash\nsleep 60", "utf-8");
		await chmod(scriptPath, 0o755);

		const hook = makeHook({ id: "slow", path: scriptPath });
		const result = await executeHook(hook, {
			cwd: tempDir,
			timeout: 500, // 500ms timeout
		});

		assert.strictEqual(result.success, false);
		assert.strictEqual(result.exitCode, 124); // timeout convention
	});

	it("passes DISCOBOT_CHANGED_FILES env var", async () => {
		const scriptPath = join(tempDir, "env.sh");
		await writeFile(
			scriptPath,
			'#!/bin/bash\necho "$DISCOBOT_CHANGED_FILES"',
			"utf-8",
		);
		await chmod(scriptPath, 0o755);

		const hook = makeHook({ id: "env", path: scriptPath });
		const result = await executeHook(hook, {
			cwd: tempDir,
			changedFiles: ["foo.go", "bar.go"],
		});

		assert.strictEqual(result.success, true);
		assert.ok(result.output.includes("foo.go bar.go"));
	});

	it("passes DISCOBOT_HOOK_TYPE env var", async () => {
		const scriptPath = join(tempDir, "type.sh");
		await writeFile(
			scriptPath,
			'#!/bin/bash\necho "$DISCOBOT_HOOK_TYPE"',
			"utf-8",
		);
		await chmod(scriptPath, 0o755);

		const hook = makeHook({ id: "type", path: scriptPath, type: "file" });
		const result = await executeHook(hook, { cwd: tempDir });

		assert.strictEqual(result.success, true);
		assert.ok(result.output.trim() === "file");
	});

	it("saves output to file when outputPath is provided", async () => {
		const scriptPath = join(tempDir, "output.sh");
		await writeFile(scriptPath, '#!/bin/bash\necho "saved output"', "utf-8");
		await chmod(scriptPath, 0o755);

		const outputPath = join(tempDir, "output", "test.log");
		const hook = makeHook({ id: "output", path: scriptPath });
		await executeHook(hook, { cwd: tempDir, outputPath });

		const savedOutput = await readFile(outputPath, "utf-8");
		assert.ok(savedOutput.includes("saved output"));
	});

	it("handles non-existent script", async () => {
		const hook = makeHook({
			id: "missing",
			path: join(tempDir, "nonexistent.sh"),
		});
		const result = await executeHook(hook, { cwd: tempDir });

		assert.strictEqual(result.success, false);
	});
});

describe("getHookOutputPath", () => {
	it("returns correct path", () => {
		const path = getHookOutputPath(
			"/home/user/.discobot/session1/hooks",
			"my-hook",
		);
		assert.strictEqual(
			path,
			"/home/user/.discobot/session1/hooks/output/my-hook.log",
		);
	});
});
