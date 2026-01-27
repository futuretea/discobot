import assert from "node:assert";
import { describe, it } from "node:test";
import { render } from "@testing-library/react";
import { CommitStatus, SessionStatus } from "@/lib/api-constants";
import type { Session } from "@/lib/api-types";
import {
	getSessionHoverText,
	getSessionStatusIndicator,
} from "./session-utils";

const createMockSession = (
	status: string,
	commitStatus?: string,
	errorMessage?: string,
	commitError?: string,
): Session => ({
	id: "test-session",
	name: "Test Session",
	description: "",
	timestamp: new Date().toISOString(),
	status: status as Session["status"],
	commitStatus: commitStatus as Session["commitStatus"],
	errorMessage,
	commitError,
	files: [],
});

describe("getSessionHoverText", () => {
	it("should return commit error when commit failed", () => {
		const session = createMockSession(
			SessionStatus.READY,
			CommitStatus.FAILED,
			undefined,
			"Failed to apply patches",
		);
		const text = getSessionHoverText(session);
		assert.strictEqual(text, "Commit Failed: Failed to apply patches");
	});

	it("should return formatted status for error state", () => {
		const session = createMockSession(
			SessionStatus.ERROR,
			undefined,
			"Sandbox creation failed",
		);
		const text = getSessionHoverText(session);
		assert.strictEqual(text, "Error: Sandbox creation failed");
	});

	it("should return formatted status for running state", () => {
		const session = createMockSession(SessionStatus.RUNNING);
		const text = getSessionHoverText(session);
		assert.strictEqual(text, "Running");
	});

	it("should format status with underscores", () => {
		const session = createMockSession(SessionStatus.PULLING_IMAGE);
		const text = getSessionHoverText(session);
		assert.strictEqual(text, "Pulling Image");
	});
});

describe("getSessionStatusIndicator", () => {
	it("should show blue spinner for running state (default size)", () => {
		const session = createMockSession(SessionStatus.RUNNING);
		const { container } = render(getSessionStatusIndicator(session));
		const svg = container.querySelector("svg");
		assert.ok(svg, "Should render an SVG icon");
		assert.ok(
			svg.classList.contains("text-blue-500"),
			"Should have blue color",
		);
		assert.ok(svg.classList.contains("animate-spin"), "Should be spinning");
		assert.ok(
			svg.classList.contains("h-3.5") || svg.classList.contains("w-3.5"),
			"Should have default size",
		);
	});

	it("should show blue spinner for running state (small size)", () => {
		const session = createMockSession(SessionStatus.RUNNING);
		const { container } = render(getSessionStatusIndicator(session, "small"));
		const svg = container.querySelector("svg");
		assert.ok(svg, "Should render an SVG icon");
		assert.ok(
			svg.classList.contains("text-blue-500"),
			"Should have blue color",
		);
		assert.ok(svg.classList.contains("animate-spin"), "Should be spinning");
		assert.ok(
			svg.classList.contains("h-2.5") || svg.classList.contains("w-2.5"),
			"Should have small size",
		);
	});

	it("should show green circle for ready state", () => {
		const session = createMockSession(SessionStatus.READY);
		const { container } = render(getSessionStatusIndicator(session));
		const svg = container.querySelector("svg");
		assert.ok(svg, "Should render an SVG icon");
		assert.ok(
			svg.classList.contains("text-green-500"),
			"Should have green color",
		);
		assert.ok(svg.classList.contains("fill-green-500"), "Should be filled");
	});

	it("should prioritize commit status over session status", () => {
		const session = createMockSession(
			SessionStatus.READY,
			CommitStatus.COMMITTING,
		);
		const { container } = render(getSessionStatusIndicator(session));
		const svg = container.querySelector("svg");
		assert.ok(svg, "Should render an SVG icon");
		assert.ok(
			svg.classList.contains("text-blue-500"),
			"Should show commit status (blue)",
		);
		assert.ok(svg.classList.contains("animate-spin"), "Should be spinning");
	});

	it("should show yellow spinner for initializing states", () => {
		const session = createMockSession(SessionStatus.INITIALIZING);
		const { container } = render(getSessionStatusIndicator(session));
		const svg = container.querySelector("svg");
		assert.ok(svg, "Should render an SVG icon");
		assert.ok(
			svg.classList.contains("text-yellow-500"),
			"Should have yellow color",
		);
		assert.ok(svg.classList.contains("animate-spin"), "Should be spinning");
	});

	it("should show pause icon for stopped state", () => {
		const session = createMockSession(SessionStatus.STOPPED);
		const { container } = render(getSessionStatusIndicator(session));
		const svg = container.querySelector("svg");
		assert.ok(svg, "Should render an SVG icon");
		assert.ok(
			svg.classList.contains("text-muted-foreground"),
			"Should have muted color",
		);
	});

	it("should show error icon for error state", () => {
		const session = createMockSession(SessionStatus.ERROR);
		const { container } = render(getSessionStatusIndicator(session));
		const svg = container.querySelector("svg");
		assert.ok(svg, "Should render an SVG icon");
		assert.ok(
			svg.classList.contains("text-destructive"),
			"Should have destructive color",
		);
	});

	it("should show check icon for completed commit", () => {
		const session = createMockSession(
			SessionStatus.READY,
			CommitStatus.COMPLETED,
		);
		const { container } = render(getSessionStatusIndicator(session));
		const svg = container.querySelector("svg");
		assert.ok(svg, "Should render an SVG icon");
		assert.ok(
			svg.classList.contains("text-green-500"),
			"Should have green color",
		);
	});

	it("should handle unknown status with default icon", () => {
		const session = createMockSession("unknown-status");
		const { container } = render(getSessionStatusIndicator(session));
		const svg = container.querySelector("svg");
		assert.ok(svg, "Should render an SVG icon");
		assert.ok(
			svg.classList.contains("text-muted-foreground"),
			"Should have muted color for unknown status",
		);
	});
});
