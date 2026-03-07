import assert from "node:assert";
import { describe, it } from "node:test";
import { render } from "@testing-library/react";
import { ModeSelector } from "./mode-selector";

describe("ModeSelector", () => {
	it('shows Build when selectedMode is "build"', () => {
		const view = render(
			<ModeSelector selectedMode="build" onSelectMode={() => {}} />,
		);

		assert.ok(view.getByRole("button", { name: /build/i }));
		view.unmount();
	});

	it("shows Build when selectedMode is an empty string", () => {
		const view = render(
			<ModeSelector selectedMode="" onSelectMode={() => {}} />,
		);

		assert.ok(view.getByRole("button", { name: /build/i }));
		view.unmount();
	});
});
