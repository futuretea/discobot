/**
 * Tauri utilities for cross-platform functionality.
 *
 * These utilities provide consistent behavior between Tauri (desktop app)
 * and browser environments.
 */

/**
 * Whether the app is running in Tauri (desktop) mode.
 * Checks for Tauri runtime API or build-time env var.
 */
export const IS_TAURI =
	typeof window !== "undefined" &&
	("__TAURI__" in window ||
		import.meta.env.VITE_TAURI === "true" ||
		import.meta.env.TAURI_ENV_PLATFORM !== undefined);

/**
 * Open a URL in the system's default browser.
 *
 * In Tauri, this uses the opener plugin to launch the external browser.
 * In browser mode, this uses window.open().
 *
 * @param url - The URL to open
 */
export async function openExternal(url: string): Promise<void> {
	if (IS_TAURI) {
		const { openUrl: tauriOpenUrl } = await import("@tauri-apps/plugin-opener");
		await tauriOpenUrl(url);
	} else {
		window.open(url, "_blank", "noopener,noreferrer");
	}
}

/**
 * Open a URL handler (like vscode://, cursor://, etc.).
 *
 * In Tauri, this uses the opener plugin's openUrl function.
 * In browser mode, this uses window.location.href.
 *
 * @param url - The URL to open (can be a custom protocol like vscode://)
 */
export async function openUrl(url: string): Promise<void> {
	if (IS_TAURI) {
		const { openUrl: tauriOpenUrl } = await import("@tauri-apps/plugin-opener");
		await tauriOpenUrl(url);
	} else {
		// In browser mode, use location.href for custom protocols
		window.location.href = url;
	}
}
