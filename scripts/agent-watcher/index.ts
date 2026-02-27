#!/usr/bin/env npx tsx
/**
 * Agent Image Watcher - Entry point
 *
 * Watches the ./agent-api, ./agent directories and ./Dockerfile for changes
 * and automatically rebuilds the Docker image, then updates server/.env with
 * the new image digest.
 *
 * Usage: npx tsx scripts/agent-watcher/index.ts
 */

import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { AgentWatcher } from "./watcher.js";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT_DIR = join(__dirname, "../..");
const AGENT_API_DIR = join(ROOT_DIR, "agent-api");
const AGENT_DIR = join(ROOT_DIR, "agent");
const SERVER_ENV_PATH = join(ROOT_DIR, "server", ".env");

/** Parse a .env file into a key/value map. Silently returns {} if missing. */
async function loadDotEnv(path: string): Promise<Record<string, string>> {
	try {
		const content = await readFile(path, "utf-8");
		const result: Record<string, string> = {};
		for (const line of content.split("\n")) {
			const trimmed = line.trim();
			if (!trimmed || trimmed.startsWith("#")) continue;
			const eqIdx = trimmed.indexOf("=");
			if (eqIdx === -1) continue;
			result[trimmed.slice(0, eqIdx).trim()] = trimmed.slice(eqIdx + 1).trim();
		}
		return result;
	} catch {
		return {};
	}
}

const localEnv = await loadDotEnv(join(__dirname, ".env"));
// process.env takes priority over .env file; default to "runtime-shell"
const buildTarget =
	process.env.SANDBOX_TARGET ?? localEnv.SANDBOX_TARGET ?? "runtime-shell";

const watcher = new AgentWatcher({
	agentDir: AGENT_API_DIR,
	additionalDirs: [AGENT_DIR],
	projectRoot: ROOT_DIR,
	envFilePath: SERVER_ENV_PATH,
	imageName: "discobot-agent-api",
	imageTag: "dev",
	buildTarget,
	debounceMs: 500,
});

watcher.logger.log(`Build target: ${buildTarget}`);
watcher.start().catch((err) => {
	console.error(`Fatal error: ${err}`);
	process.exit(1);
});
