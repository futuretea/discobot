import "./globals.css";

import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { ErrorBoundary } from "@/components/error-boundary";
import { api } from "@/lib/api-client";
import { initServerConfig, initTauriConfig } from "@/lib/api-config";
import { App } from "./App";

const root = document.getElementById("root");
if (!root) throw new Error("Root element not found");

// Health check with retry logic
async function waitForBackend(maxRetries = 30, delayMs = 1000): Promise<void> {
	let lastError: Error | null = null;

	for (let attempt = 0; attempt < maxRetries; attempt++) {
		try {
			await api.getSystemStatus();
			return; // Success
		} catch (error) {
			lastError = error as Error;
			if (attempt < maxRetries - 1) {
				await new Promise((resolve) => setTimeout(resolve, delayMs));
			}
		}
	}

	throw new Error(
		`Backend health check failed after ${maxRetries} attempts: ${lastError?.message || "Unknown error"}`,
	);
}

// Initialize Tauri config and wait for backend before rendering
async function initializeApp() {
	try {
		// Step 1: Initialize Tauri config (if in Tauri mode)
		await initTauriConfig();

		// Step 2: Wait for backend to be ready
		await waitForBackend();

		// Step 3: Fetch server config (SSH port, etc.)
		await initServerConfig();

		// Step 4: Remove loading screen
		const loadingScreen = document.getElementById("loading-screen");
		if (loadingScreen) {
			loadingScreen.remove();
		}

		// Step 5: Render app (root is guaranteed to exist from check at line 11)
		// biome-ignore lint/style/noNonNullAssertion: root is checked at module level
		createRoot(root!).render(
			<ErrorBoundary>
				<BrowserRouter>
					<App />
				</BrowserRouter>
			</ErrorBoundary>,
		);
	} catch (error) {
		// Show error in loading screen
		const loadingScreen = document.getElementById("loading-screen");
		if (loadingScreen) {
			loadingScreen.innerHTML = `
				<div style="
					display: flex;
					flex-direction: column;
					align-items: center;
					gap: 16px;
					color: #ef4444;
					text-align: center;
					padding: 32px;
				">
					<div style="font-size: 24px; font-weight: 600;">Failed to connect to backend</div>
					<div style="font-size: 14px; opacity: 0.8; max-width: 500px;">
						${error instanceof Error ? error.message : String(error)}
					</div>
					<button
						onclick="location.reload()"
						style="
							margin-top: 16px;
							padding: 8px 16px;
							background: #ef4444;
							color: white;
							border: none;
							border-radius: 6px;
							cursor: pointer;
							font-size: 14px;
							font-weight: 500;
						"
					>
						Retry
					</button>
				</div>
			`;
		}
		throw error;
	}
}

initializeApp();
