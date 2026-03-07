import assert from "node:assert";
import { afterEach, describe, it } from "node:test";
import {
	cleanup,
	fireEvent,
	render,
	screen,
	within,
} from "@testing-library/react";
import type { UIMessage } from "ai";
import { CompactMessageParts } from "./compact-message-parts";

function createTextPart(
	text: string,
	state: "streaming" | "done" = "done",
): UIMessage["parts"][number] {
	return {
		type: "text",
		text,
		state,
	};
}

function createMessage(
	overrides: Partial<UIMessage> & { parts: UIMessage["parts"] },
): UIMessage {
	return {
		id: overrides.id ?? "msg-1",
		role: overrides.role ?? "assistant",
		parts: overrides.parts,
	};
}

describe("CompactMessageParts", () => {
	afterEach(() => {
		cleanup();
	});

	it("collapses earlier parts during streaming, showing only the latest two", () => {
		const message = createMessage({
			parts: [
				createTextPart("Step 1"),
				createTextPart("Step 2"),
				createTextPart("Step 3"),
				createTextPart("Step 4", "streaming"),
			],
		});

		const { container } = render(
			<CompactMessageParts message={message} isStreaming />,
		);

		const collapsibleContent = container.querySelector(
			'[data-slot="collapsible-content"]',
		);
		if (!(collapsibleContent instanceof HTMLElement)) {
			throw new Error("collapsible summary should render");
		}
		assert.strictEqual(
			collapsibleContent.getAttribute("data-state"),
			"closed",
			"summary should be collapsed by default",
		);

		const trigger = container.querySelector(
			'[data-slot="collapsible-trigger"]',
		);
		if (!(trigger instanceof HTMLElement)) {
			throw new Error("summary trigger should render");
		}
		fireEvent.click(trigger);

		const summary = within(collapsibleContent);
		assert.ok(summary.getByText("Step 1"));
		assert.ok(summary.getByText("Step 2"));
		assert.strictEqual(
			summary.queryByText("Step 3"),
			null,
			"third step should not appear in the collapsed summary",
		);

		assert.ok(screen.getByText("Step 3"));
		assert.ok(screen.getByText("Step 4"));
	});

	it("keeps only the last part expanded after streaming completes", () => {
		const message = createMessage({
			parts: [
				createTextPart("Step 1"),
				createTextPart("Step 2"),
				createTextPart("Step 3"),
				createTextPart("Step 4"),
			],
		});

		const { container } = render(
			<CompactMessageParts message={message} isStreaming={false} />,
		);

		const collapsibleContent = container.querySelector(
			'[data-slot="collapsible-content"]',
		);
		if (!(collapsibleContent instanceof HTMLElement)) {
			throw new Error("collapsible summary should render");
		}

		const trigger = container.querySelector(
			'[data-slot="collapsible-trigger"]',
		);
		if (!(trigger instanceof HTMLElement)) {
			throw new Error("summary trigger should render");
		}
		fireEvent.click(trigger);

		const summary = within(collapsibleContent);
		assert.ok(summary.getByText("Step 1"));
		assert.ok(summary.getByText("Step 2"));
		assert.ok(summary.getByText("Step 3"));
		assert.strictEqual(
			summary.queryByText("Step 4"),
			null,
			"last step should remain outside the summary",
		);

		assert.ok(screen.getByText("Step 4"));
	});

	it("does not compact streaming user messages", () => {
		const message = createMessage({
			role: "user",
			parts: [
				createTextPart("User step 1", "streaming"),
				createTextPart("User step 2", "streaming"),
				createTextPart("User step 3", "streaming"),
			],
		});

		const { container } = render(
			<CompactMessageParts message={message} isStreaming />,
		);

		const summary = container.querySelector('[data-slot="collapsible"]');
		assert.strictEqual(summary, null, "user messages should not compact");

		assert.ok(screen.getByText("User step 1"));
		assert.ok(screen.getByText("User step 2"));
		assert.ok(screen.getByText("User step 3"));
	});
});
