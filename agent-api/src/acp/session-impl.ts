import type { UIMessage } from "ai";
import type { Session } from "../agent/session.js";

/**
 * Implementation of Session interface for ACP client.
 * Each session maintains its own message history.
 */
export class SessionImpl implements Session {
	private messages: UIMessage[] = [];

	constructor(public readonly id: string) {}

	getMessages(): UIMessage[] {
		return this.messages;
	}

	addMessage(message: UIMessage): void {
		this.messages.push(message);
	}

	updateMessage(id: string, updates: Partial<UIMessage>): void {
		const index = this.messages.findIndex((m) => m.id === id);
		if (index !== -1) {
			this.messages[index] = { ...this.messages[index], ...updates };
		}
	}

	getLastAssistantMessage(): UIMessage | undefined {
		for (let i = this.messages.length - 1; i >= 0; i--) {
			if (this.messages[i].role === "assistant") {
				return this.messages[i];
			}
		}
		return undefined;
	}

	clearMessages(): void {
		this.messages = [];
	}
}
