/**
 * ChatPanel Re-render Performance Test
 *
 * Uses Node's built-in test runner with React Profiler to verify
 * that ChatPanel doesn't re-render excessively when typing.
 *
 * Run with:
 *   node --import ./test/setup.js --import tsx --test components/ide/chat-panel.test.tsx
 *
 * Setup:
 * - test/setup.js initializes jsdom globals BEFORE React/testing-library load
 * - Uses actual providers wrapped in SWRConfig to prevent API calls
 * - React Profiler tracks render counts
 */

import assert from "node:assert";
import { afterEach, describe, test } from "node:test";
import { act, cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Profiler, type ProfilerOnRenderCallback } from "react";
import { SWRConfig } from "swr";
import { DialogProvider } from "../../lib/contexts/dialog-context.js";
// Import the actual providers and components
import { SessionProvider } from "../../lib/contexts/session-context.js";
import { ChatPanel } from "./chat-panel.js";

// Wrapper component that provides all necessary contexts
function TestWrapper({ children }: { children: React.ReactNode }) {
	return (
		<SWRConfig
			value={{
				provider: () => new Map(),
				dedupingInterval: 0,
				revalidateOnFocus: false,
				revalidateOnReconnect: false,
				fetcher: () =>
					Promise.resolve({ workspaces: [], agents: [], agentTypes: [] }),
			}}
		>
			<SessionProvider>
				<DialogProvider>{children}</DialogProvider>
			</SessionProvider>
		</SWRConfig>
	);
}

describe("ChatPanel", () => {
	afterEach(() => {
		cleanup();
	});

	describe("re-render performance", () => {
		test("should track re-renders when typing (baseline test)", async () => {
			const user = userEvent.setup({ delay: null });
			const renderCounts: Record<string, number> = {};

			const onRender: ProfilerOnRenderCallback = (id) => {
				renderCounts[id] = (renderCounts[id] || 0) + 1;
			};

			render(
				<TestWrapper>
					<Profiler id="ChatPanel" onRender={onRender}>
						<ChatPanel />
					</Profiler>
				</TestWrapper>,
			);

			// Wait for initial effects to settle
			await act(async () => {
				await new Promise((resolve) => setTimeout(resolve, 100));
			});

			const countAfterSetup = renderCounts.ChatPanel;
			const textarea = screen.getByPlaceholderText(
				/what would you like to work on/i,
			);

			// Type "hello" (5 characters)
			await user.type(textarea, "hello");

			await act(async () => {
				await new Promise((resolve) => setTimeout(resolve, 50));
			});

			const countAfterTyping = renderCounts.ChatPanel;
			const rendersFromTyping = countAfterTyping - countAfterSetup;

			console.log(
				`[ChatPanel render test] setup=${countAfterSetup}, afterTyping=${countAfterTyping}, fromTyping=${rendersFromTyping}`,
			);

			// ChatPanel should render at least once on mount
			assert.ok(
				countAfterSetup >= 1,
				`ChatPanel should render at least once on mount (got ${countAfterSetup})`,
			);

			// Typing should NOT cause ChatPanel to re-render
			// Input state is managed in the Input component to prevent parent re-renders
			// Current baseline: 5 re-renders from provider context updates (to be investigated)
			// This test catches regressions if re-renders increase beyond the baseline
			const MAX_ALLOWED_RERENDERS = 5;
			assert.ok(
				rendersFromTyping <= MAX_ALLOWED_RERENDERS,
				`ChatPanel should not re-render excessively when typing (got ${rendersFromTyping} re-renders, max allowed: ${MAX_ALLOWED_RERENDERS})`,
			);
		});
	});
});
