/**
 * Hook Front Matter Parser
 *
 * Discovers and parses hook files from .discobot/hooks/.
 * Reuses the same front matter format as services (YAML between #--- delimiters).
 *
 * Hook types:
 * - session: Run once at container startup (executed by Go agent init)
 * - file: Run at end of LLM turn when matching files change
 * - pre-commit: Installed as git pre-commit hooks
 */

import { readdir, readFile, stat } from "node:fs/promises";
import { join } from "node:path";
import { normalizeServiceId, parseFrontMatter } from "../services/parser.js";

/**
 * Valid hook types
 */
export type HookType = "session" | "file" | "pre-commit";

/**
 * Hook configuration parsed from YAML front matter
 */
export interface HookConfig {
	/** Display name (defaults to filename) */
	name?: string;
	/** Hook type (required) */
	type?: HookType;
	/** Description */
	description?: string;
	/** Run as root or user (session hooks only, default: "user") */
	runAs?: "root" | "user";
	/** Glob pattern for file matching (file hooks only, required for file hooks) */
	pattern?: string;
	/** Whether to notify the LLM on failure (file/pre-commit hooks, default: true) */
	notifyLlm?: boolean;
}

/**
 * Discovered hook with metadata
 */
export interface Hook {
	/** Normalized ID from filename */
	id: string;
	/** Display name (from config or filename) */
	name: string;
	/** Hook type */
	type: HookType;
	/** Description */
	description?: string;
	/** Absolute path to the hook file */
	path: string;
	/** Run as root or user (session hooks) */
	runAs: "root" | "user";
	/** Glob pattern (file hooks) */
	pattern?: string;
	/** Whether to notify the LLM on failure */
	notifyLlm: boolean;
}

const VALID_HOOK_TYPES = new Set<string>(["session", "file", "pre-commit"]);

/**
 * Parse hook-specific fields from simple YAML content.
 */
function parseHookYaml(content: string): HookConfig {
	const config: HookConfig = {};

	for (const line of content.split("\n")) {
		const trimmed = line.trim();

		if (!trimmed || trimmed.startsWith("#")) {
			continue;
		}

		const colonIndex = trimmed.indexOf(":");
		if (colonIndex === -1) {
			continue;
		}

		const key = trimmed.slice(0, colonIndex).trim();
		let value = trimmed.slice(colonIndex + 1).trim();

		// Remove quotes if present
		if (
			(value.startsWith('"') && value.endsWith('"')) ||
			(value.startsWith("'") && value.endsWith("'"))
		) {
			value = value.slice(1, -1);
		}

		switch (key) {
			case "name":
				config.name = value;
				break;
			case "type":
				if (VALID_HOOK_TYPES.has(value)) {
					config.type = value as HookType;
				}
				break;
			case "description":
				config.description = value;
				break;
			case "run_as":
				if (value === "root" || value === "user") {
					config.runAs = value;
				}
				break;
			case "pattern":
				config.pattern = value;
				break;
			case "notify_llm": {
				const lower = value.toLowerCase();
				if (lower === "false" || lower === "no" || lower === "0") {
					config.notifyLlm = false;
				} else {
					config.notifyLlm = true;
				}
				break;
			}
		}
	}

	return config;
}

/**
 * Parse a hook file and extract its configuration.
 *
 * Uses the same front matter parser as services but extracts hook-specific fields.
 * The parseFrontMatter function handles delimiter detection and YAML extraction;
 * we then re-parse the raw YAML lines for hook fields since the service parser
 * only knows about service-specific keys.
 */
export function parseHookFrontMatter(content: string): {
	config: HookConfig;
	hasShebang: boolean;
} {
	const result = parseFrontMatter(content);

	// Re-parse the YAML section for hook-specific fields.
	// We need the raw YAML content, so we extract it from the file content
	// using the same delimiter detection logic.
	const lines = content.split("\n");
	const hasShebang = lines[0]?.startsWith("#!") ?? false;
	const frontMatterStartLine = hasShebang ? 1 : 0;

	// Find delimiter lines and extract YAML between them
	let yamlContent = "";
	const startLine = lines[frontMatterStartLine];
	if (startLine) {
		const trimmed = startLine.trim();
		let prefix = "";
		let delimiter = "";

		if (trimmed === "---") {
			prefix = "";
			delimiter = "---";
		} else if (trimmed === "#---") {
			prefix = "#";
			delimiter = "#---";
		} else if (trimmed === "//---") {
			prefix = "//";
			delimiter = "//---";
		}

		if (delimiter) {
			const yamlLines: string[] = [];
			for (let i = frontMatterStartLine + 1; i < lines.length; i++) {
				if (lines[i].trim() === delimiter) {
					break;
				}
				let line = lines[i];
				if (prefix) {
					const prefixIndex = line.indexOf(prefix);
					if (prefixIndex !== -1) {
						line = line.slice(prefixIndex + prefix.length).trimStart();
					}
				}
				yamlLines.push(line);
			}
			yamlContent = yamlLines.join("\n");
		}
	}

	const config = parseHookYaml(yamlContent);

	// Carry over name from service parser if not set in hook parser
	if (!config.name && result.config.name) {
		config.name = result.config.name;
	}

	return { config, hasShebang: result.hasShebang };
}

/**
 * Discover all hooks in the hooks directory.
 *
 * Hooks must be:
 * - Regular files (not directories, not hidden)
 * - Executable
 * - Have a shebang line
 * - Have front matter with a valid `type` field
 * - File hooks must have a `pattern` field
 */
export async function discoverHooks(hooksDir: string): Promise<Hook[]> {
	const hooks: Hook[] = [];

	try {
		const entries = await readdir(hooksDir, { withFileTypes: true });

		for (const entry of entries) {
			if (entry.isDirectory() || entry.name.startsWith(".")) {
				continue;
			}

			const filePath = join(hooksDir, entry.name);

			try {
				const fileStat = await stat(filePath);
				const isExecutable = (fileStat.mode & 0o111) !== 0;
				if (!isExecutable) {
					continue;
				}

				const content = await readFile(filePath, "utf-8");
				const { config, hasShebang } = parseHookFrontMatter(content);

				if (!hasShebang) {
					continue;
				}

				if (!config.type) {
					continue;
				}

				// File hooks require a pattern
				if (config.type === "file" && !config.pattern) {
					continue;
				}

				const hookId = normalizeServiceId(entry.name);

				hooks.push({
					id: hookId,
					name: config.name || hookId,
					type: config.type,
					description: config.description,
					path: filePath,
					runAs: config.runAs || "user",
					pattern: config.pattern,
					notifyLlm: config.notifyLlm !== false, // default true
				});
			} catch {
				// Skip files that can't be read
			}
		}
	} catch {
		// Directory doesn't exist - return empty list
		return [];
	}

	// Sort by name for deterministic order
	hooks.sort((a, b) => a.name.localeCompare(b.name));

	return hooks;
}

/**
 * Hooks directory name within the workspace
 */
export const HOOKS_DIR = ".discobot/hooks";
