/**
 * Unit tests for hook front matter parser
 */

import assert from "node:assert";
import { describe, it } from "node:test";
import { parseHookFrontMatter } from "./parser.js";

describe("parseHookFrontMatter", () => {
	it("parses a session hook with run_as", () => {
		const content = `#!/bin/bash
#---
# name: Install deps
# type: session
# run_as: root
#---
apt-get install -y curl`;

		const { config, hasShebang } = parseHookFrontMatter(content);
		assert.strictEqual(hasShebang, true);
		assert.strictEqual(config.name, "Install deps");
		assert.strictEqual(config.type, "session");
		assert.strictEqual(config.runAs, "root");
	});

	it("parses a file hook with pattern and notify_llm", () => {
		const content = `#!/bin/bash
#---
# name: Go format
# type: file
# pattern: "*.go"
# notify_llm: true
#---
gofmt -l $DISCOBOT_CHANGED_FILES`;

		const { config } = parseHookFrontMatter(content);
		assert.strictEqual(config.name, "Go format");
		assert.strictEqual(config.type, "file");
		assert.strictEqual(config.pattern, "*.go");
		assert.strictEqual(config.notifyLlm, true);
	});

	it("parses notify_llm: false", () => {
		const content = `#!/bin/bash
#---
# name: Silent lint
# type: file
# pattern: "*.ts"
# notify_llm: false
#---
eslint --fix $DISCOBOT_CHANGED_FILES`;

		const { config } = parseHookFrontMatter(content);
		assert.strictEqual(config.notifyLlm, false);
	});

	it("parses a pre-commit hook", () => {
		const content = `#!/bin/bash
#---
# name: Type check
# type: pre-commit
#---
pnpm typecheck`;

		const { config } = parseHookFrontMatter(content);
		assert.strictEqual(config.type, "pre-commit");
		assert.strictEqual(config.name, "Type check");
	});

	it("rejects invalid hook type", () => {
		const content = `#!/bin/bash
#---
# name: Bad hook
# type: invalid
#---
echo hello`;

		const { config } = parseHookFrontMatter(content);
		assert.strictEqual(config.type, undefined);
	});

	it("handles no front matter", () => {
		const content = `#!/bin/bash
echo hello`;

		const { config, hasShebang } = parseHookFrontMatter(content);
		assert.strictEqual(hasShebang, true);
		assert.strictEqual(config.type, undefined);
		assert.strictEqual(config.name, undefined);
	});

	it("handles no shebang", () => {
		const content = `#---
# type: session
#---
echo hello`;

		const { config, hasShebang } = parseHookFrontMatter(content);
		assert.strictEqual(hasShebang, false);
		assert.strictEqual(config.type, "session");
	});

	it("handles plain delimiter style", () => {
		const content = `#!/bin/bash
---
name: Plain hook
type: session
---
echo hello`;

		const { config } = parseHookFrontMatter(content);
		assert.strictEqual(config.name, "Plain hook");
		assert.strictEqual(config.type, "session");
	});

	it("defaults run_as to undefined (caller defaults to user)", () => {
		const content = `#!/bin/bash
#---
# name: Default user
# type: session
#---
echo hello`;

		const { config } = parseHookFrontMatter(content);
		assert.strictEqual(config.runAs, undefined);
	});

	it("rejects invalid run_as values", () => {
		const content = `#!/bin/bash
#---
# name: Bad run_as
# type: session
# run_as: admin
#---
echo hello`;

		const { config } = parseHookFrontMatter(content);
		assert.strictEqual(config.runAs, undefined);
	});

	it("handles pattern with braces", () => {
		const content = `#!/bin/bash
#---
# name: Multi ext
# type: file
# pattern: "*.{ts,tsx}"
#---
echo check`;

		const { config } = parseHookFrontMatter(content);
		assert.strictEqual(config.pattern, "*.{ts,tsx}");
	});

	it("handles pattern with double star", () => {
		const content = `#!/bin/bash
#---
# name: Deep match
# type: file
# pattern: "src/**/*.go"
#---
echo check`;

		const { config } = parseHookFrontMatter(content);
		assert.strictEqual(config.pattern, "src/**/*.go");
	});

	it("handles notify_llm with various false-like values", () => {
		for (const falseValue of ["false", "no", "0", "False", "NO"]) {
			const content = `#!/bin/bash
#---
# type: file
# pattern: "*.ts"
# notify_llm: ${falseValue}
#---
echo check`;

			const { config } = parseHookFrontMatter(content);
			assert.strictEqual(
				config.notifyLlm,
				false,
				`Expected false for notify_llm: ${falseValue}`,
			);
		}
	});

	it("handles empty content", () => {
		const { config, hasShebang } = parseHookFrontMatter("");
		assert.strictEqual(hasShebang, false);
		assert.strictEqual(config.type, undefined);
	});
});
