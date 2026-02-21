/**
 * OpenCode Session implementation.
 *
 * Fetches messages from the OpenCode server on load() and caches them.
 * OpenCode handles all persistence — we just read what the server has.
 *
 * Design:
 * - Call load() to fetch messages from OpenCode server into cache
 * - getMessages() returns the cached snapshot
 * - Call load() again at start of each turn to refresh
 */

import type { OpencodeClient } from "@opencode-ai/sdk/v2";
import type { UIMessage } from "ai";
import type { Session } from "../agent/session.js";
import { translateOpenCodeMessageToUIMessage } from "./translate.js";

export class OpenCodeSession implements Session {
	private cachedMessages: UIMessage[] = [];
	private opencodeSessionId: string | null = null;

	constructor(public readonly id: string) {}

	/**
	 * Load messages from the OpenCode server into the cache.
	 *
	 * @param client - The OpenCode API client
	 * @param opencodeSessionId - The OpenCode session ID to load from.
	 *   This may differ from this.id (the discobot session ID).
	 */
	async load(
		client: OpencodeClient,
		opencodeSessionId?: string,
	): Promise<void> {
		const sessionIdToLoad =
			opencodeSessionId ?? this.opencodeSessionId ?? this.id;
		if (opencodeSessionId) {
			this.opencodeSessionId = opencodeSessionId;
		}

		const result = await client.session.messages({
			sessionID: sessionIdToLoad,
		});

		if (result.error || !result.data) {
			return;
		}

		console.log(
			`[opencode] session.load: ${result.data.length} messages from OpenCode`,
		);
		for (const msg of result.data) {
			console.log(
				`[opencode] raw message: ${JSON.stringify({ info: msg.info, parts: msg.parts })}`,
			);
		}

		this.cachedMessages = result.data
			.map((msg) => translateOpenCodeMessageToUIMessage(msg.info, msg.parts))
			.filter((msg) => msg.parts.length > 0);

		console.log(
			`[opencode] translated: ${JSON.stringify(this.cachedMessages)}`,
		);
	}

	/**
	 * Get all messages (returns cached snapshot from last load).
	 */
	getMessages(): UIMessage[] {
		return this.cachedMessages;
	}

	/**
	 * Clear cached messages.
	 */
	clearMessages(): void {
		this.cachedMessages = [];
		this.opencodeSessionId = null;
	}

	/**
	 * Set the OpenCode session ID for loading.
	 */
	setOpencodeSessionId(opencodeSessionId: string): void {
		this.opencodeSessionId = opencodeSessionId;
	}
}
