/**
 * ChatPanel Error Handling Tests
 *
 * Tests that error banner clears when SWR successfully refetches messages
 * after a server restart or session reinitialization.
 *
 * Run with:
 *   node --test components/ide/chat-panel-error-handling.test.js
 */

import assert from "node:assert";
import { describe, test } from "node:test";

/**
 * Simulates the error display logic from ChatPanel
 * Extracted to test the core logic without needing to mock the entire component
 */
function getErrorDisplayState(chatStatus, swrError, resume) {
	// Only show error if chat has error AND (SWR also has error OR it's a new session)
	// This ensures error clears when SWR successfully refetches after server restart
	const hasError = chatStatus === "error" && (!resume || !!swrError);

	return { hasError };
}

describe("ChatPanel - Error Display Logic", () => {
	describe("New session errors (resume=false)", () => {
		test("should show error when chat has error in new session", () => {
			const { hasError } = getErrorDisplayState("error", null, false);
			assert.strictEqual(
				hasError,
				true,
				"Error should show for new session even without SWR error",
			);
		});

		test("should show error when both chat and SWR have errors in new session", () => {
			const { hasError } = getErrorDisplayState(
				"error",
				new Error("Network error"),
				false,
			);
			assert.strictEqual(
				hasError,
				true,
				"Error should show when both chat and SWR have errors",
			);
		});

		test("should not show error when chat is idle in new session", () => {
			const { hasError } = getErrorDisplayState("idle", null, false);
			assert.strictEqual(
				hasError,
				false,
				"Error should not show when chat status is idle",
			);
		});
	});

	describe("Existing session errors (resume=true)", () => {
		test("should show error when both chat and SWR have errors", () => {
			const { hasError } = getErrorDisplayState(
				"error",
				new Error("Network error"),
				true,
			);
			assert.strictEqual(
				hasError,
				true,
				"Error should show when both chat and SWR have errors",
			);
		});

		test("should NOT show error when chat has error but SWR succeeds", () => {
			// This is the key fix: when SWR successfully refetches (no swrError),
			// the error banner should disappear even if chatStatus is still "error"
			const { hasError } = getErrorDisplayState("error", null, true);
			assert.strictEqual(
				hasError,
				false,
				"Error should NOT show when SWR refetch succeeds (swrError is null)",
			);
		});

		test("should not show error when chat is idle", () => {
			const { hasError } = getErrorDisplayState("idle", null, true);
			assert.strictEqual(
				hasError,
				false,
				"Error should not show when chat status is idle",
			);
		});

		test("should not show error when chat is streaming", () => {
			const { hasError } = getErrorDisplayState("streaming", null, true);
			assert.strictEqual(
				hasError,
				false,
				"Error should not show when chat is actively streaming",
			);
		});
	});

	describe("Server restart scenario", () => {
		test("should handle error clearing after server restart", () => {
			// Step 1: Server is down, both chat and SWR have errors
			let chatStatus = "error";
			let swrError = new Error("Connection refused");
			const resume = true;

			let result = getErrorDisplayState(chatStatus, swrError, resume);
			assert.strictEqual(
				result.hasError,
				true,
				"Step 1: Error should show when server is down",
			);

			// Step 2: User clicks refresh, SWR refetch happens
			// Server is still down, so SWR still has error
			result = getErrorDisplayState(chatStatus, swrError, resume);
			assert.strictEqual(
				result.hasError,
				true,
				"Step 2: Error should persist if server still down",
			);

			// Step 3: Server restarts and reinitializes session
			// User clicks refresh again, this time SWR fetch succeeds
			swrError = null;

			result = getErrorDisplayState(chatStatus, swrError, resume);
			assert.strictEqual(
				result.hasError,
				false,
				"Step 3: Error should clear when SWR refetch succeeds",
			);

			// Step 4: User continues chatting, chat status becomes idle
			chatStatus = "idle";
			result = getErrorDisplayState(chatStatus, swrError, resume);
			assert.strictEqual(
				result.hasError,
				false,
				"Step 4: Error should remain cleared",
			);
		});

		test("should auto-clear when SWR revalidates successfully", () => {
			// Simulates the scenario where user doesn't click refresh
			// but SWR auto-revalidates on its own and succeeds

			// Initial state: error
			const chatStatus = "error";
			let swrError = new Error("Connection refused");
			const resume = true;

			let result = getErrorDisplayState(chatStatus, swrError, resume);
			assert.strictEqual(result.hasError, true, "Initial: error is shown");

			// SWR auto-revalidates (e.g., after 30s) and succeeds
			swrError = null;

			result = getErrorDisplayState(chatStatus, swrError, resume);
			assert.strictEqual(
				result.hasError,
				false,
				"Error should auto-clear when SWR succeeds",
			);
		});
	});

	describe("Edge cases", () => {
		test("should handle transition from new session to existing session", () => {
			// Start as new session with error
			let resume = false;
			const chatStatus = "error";
			const swrError = null;

			let result = getErrorDisplayState(chatStatus, swrError, resume);
			assert.strictEqual(
				result.hasError,
				true,
				"New session error should show",
			);

			// Transition to existing session (after first message sent)
			resume = true;

			result = getErrorDisplayState(chatStatus, swrError, resume);
			assert.strictEqual(
				result.hasError,
				false,
				"Error should not show in resume mode without SWR error",
			);
		});

		test("should handle SWR error without chat error", () => {
			// Edge case: SWR has an error but chat doesn't
			const { hasError } = getErrorDisplayState(
				"idle",
				new Error("SWR error"),
				true,
			);
			assert.strictEqual(
				hasError,
				false,
				"Should not show error if chat status is not error",
			);
		});

		test("should handle all chat statuses in resume mode with SWR error", () => {
			const swrError = new Error("Network error");
			const resume = true;

			const statuses = ["idle", "streaming", "submitted", "error"];

			for (const status of statuses) {
				const { hasError } = getErrorDisplayState(status, swrError, resume);
				if (status === "error") {
					assert.strictEqual(
						hasError,
						true,
						`Should show error when status=${status} with SWR error`,
					);
				} else {
					assert.strictEqual(
						hasError,
						false,
						`Should not show error when status=${status} even with SWR error`,
					);
				}
			}
		});
	});
});
