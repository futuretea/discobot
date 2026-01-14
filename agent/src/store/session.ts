import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import type { UIMessage } from "ai";

const SESSION_FILE = process.env.SESSION_FILE || "/tmp/agent-session.json";

export interface SessionData {
	sessionId: string;
	cwd: string;
	createdAt: string;
}

// In-memory message store using AI SDK's UIMessage type
let messages: UIMessage[] = [];
let sessionData: SessionData | null = null;

export function getMessages(): UIMessage[] {
	return messages;
}

export function addMessage(message: UIMessage): void {
	messages.push(message);
}

export function updateMessage(id: string, updates: Partial<UIMessage>): void {
	const index = messages.findIndex((m) => m.id === id);
	if (index !== -1) {
		messages[index] = { ...messages[index], ...updates };
	}
}

export function getLastAssistantMessage(): UIMessage | undefined {
	for (let i = messages.length - 1; i >= 0; i--) {
		if (messages[i].role === "assistant") {
			return messages[i];
		}
	}
	return undefined;
}

export function clearMessages(): void {
	messages = [];
}

export function getSessionData(): SessionData | null {
	return sessionData;
}

export async function loadSession(): Promise<SessionData | null> {
	try {
		if (!existsSync(SESSION_FILE)) {
			return null;
		}
		const content = await readFile(SESSION_FILE, "utf-8");
		sessionData = JSON.parse(content) as SessionData;
		return sessionData;
	} catch (error) {
		console.error("Failed to load session:", error);
		return null;
	}
}

export async function saveSession(data: SessionData): Promise<void> {
	try {
		const dir = dirname(SESSION_FILE);
		if (!existsSync(dir)) {
			await mkdir(dir, { recursive: true });
		}
		await writeFile(SESSION_FILE, JSON.stringify(data, null, 2), "utf-8");
		sessionData = data;
		console.log(`Session saved to ${SESSION_FILE}`);
	} catch (error) {
		console.error("Failed to save session:", error);
		throw error;
	}
}

export async function clearSession(): Promise<void> {
	try {
		if (existsSync(SESSION_FILE)) {
			const { unlink } = await import("node:fs/promises");
			await unlink(SESSION_FILE);
			console.log("Session cleared");
		}
		sessionData = null;
		messages = [];
	} catch (error) {
		console.error("Failed to clear session:", error);
	}
}
