/**
 * Tauri utilities for cross-platform functionality.
 *
 * These utilities provide consistent behavior between Tauri (desktop app)
 * and browser environments.
 */

import { isTauri } from "./api-config";

/**
 * Read plain text from the system clipboard.
 *
 * In Tauri, uses the clipboard-manager plugin which has reliable access to
 * the native clipboard on all platforms.
 * In browser mode, falls back to the Web Clipboard API (navigator.clipboard.readText).
 *
 * @returns The clipboard text, or an empty string if unavailable.
 */
export async function readClipboardText(): Promise<string> {
	if (isTauri()) {
		const { readText } = await import("@tauri-apps/plugin-clipboard-manager");
		return (await readText()) ?? "";
	}
	return navigator.clipboard.readText();
}

/**
 * Open a URL using the appropriate method for the environment and protocol.
 *
 * In Tauri, this always uses the opener plugin.
 * In browser mode:
 *   - http/https URLs use window.open() to open a new tab
 *   - Custom protocol URLs (vscode://, cursor://, etc.) use window.location.href
 *
 * @param url - The URL to open
 */
export async function openUrl(url: string): Promise<void> {
	if (isTauri()) {
		const { openUrl: tauriOpenUrl } = await import("@tauri-apps/plugin-opener");
		await tauriOpenUrl(url);
	} else if (url.startsWith("http://") || url.startsWith("https://")) {
		window.open(url, "_blank", "noopener,noreferrer");
	} else {
		window.location.href = url;
	}
}
