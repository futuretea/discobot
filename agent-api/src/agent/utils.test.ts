import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { createUIMessage, generateMessageId } from "./utils.js";

describe("agent/utils.ts", () => {
	describe("generateMessageId", () => {
		it("generates unique IDs", () => {
			const id1 = generateMessageId();
			const id2 = generateMessageId();

			assert.notEqual(id1, id2);
		});

		it("generates IDs with correct format", () => {
			const id = generateMessageId();

			assert.ok(id.startsWith("msg-"));
			assert.ok(/^msg-\d+-[a-z0-9]+$/.test(id));
		});
	});

	describe("createUIMessage", () => {
		it("creates message with generated ID", () => {
			const message = createUIMessage("user");

			assert.ok(message.id.startsWith("msg-"));
			assert.equal(message.role, "user");
			assert.deepEqual(message.parts, []);
		});

		it("creates message with provided parts", () => {
			const parts = [{ type: "text" as const, text: "Hello" }];
			const message = createUIMessage("assistant", parts);

			assert.equal(message.role, "assistant");
			assert.deepEqual(message.parts, parts);
		});

		it("always generates unique IDs", () => {
			const msg1 = createUIMessage("user");
			const msg2 = createUIMessage("user");

			assert.notEqual(msg1.id, msg2.id);
		});
	});
});
