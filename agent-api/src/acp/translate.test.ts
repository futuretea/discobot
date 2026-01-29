import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { FileUIPart } from "ai";
import { createUIMessage } from "../agent/utils.js";
import { uiMessageToContentBlocks } from "./translate.js";

describe("translate.ts", () => {
	describe("uiMessageToContentBlocks", () => {
		it("converts text parts to text ContentBlocks", () => {
			const message = createUIMessage("user", [
				{ type: "text", text: "Hello" },
			]);

			const blocks = uiMessageToContentBlocks(message);

			assert.equal(blocks.length, 1);
			assert.deepEqual(blocks[0], { type: "text", text: "Hello" });
		});

		it("converts file parts to resource_link ContentBlocks", () => {
			const filePart: FileUIPart = {
				type: "file",
				url: "data:image/png;base64,abc",
				mediaType: "image/png",
				filename: "test.png",
			};
			const message = createUIMessage("user", [filePart]);

			const blocks = uiMessageToContentBlocks(message);

			assert.equal(blocks.length, 1);
			const resourceLink = blocks[0] as {
				type: string;
				uri: string;
				name: string;
				mimeType: string;
			};
			assert.equal(resourceLink.type, "resource_link");
			assert.equal(resourceLink.uri, "data:image/png;base64,abc");
			assert.equal(resourceLink.name, "test.png");
			assert.equal(resourceLink.mimeType, "image/png");
		});

		it("uses default name when filename is missing", () => {
			const filePart: FileUIPart = {
				type: "file",
				url: "data:image/png;base64,abc",
				mediaType: "image/png",
			};
			const message = createUIMessage("user", [filePart]);

			const blocks = uiMessageToContentBlocks(message);

			const resourceLink = blocks[0] as { name: string };
			assert.equal(resourceLink.name, "file");
		});

		it("skips unsupported part types", () => {
			const message = createUIMessage("assistant", [
				{ type: "text", text: "Hello" },
				{ type: "reasoning", text: "Thinking..." },
				{
					type: "dynamic-tool",
					toolCallId: "tc-1",
					toolName: "test",
					state: "output-available",
					input: {},
					output: "result",
				},
			]);

			const blocks = uiMessageToContentBlocks(message);

			// Only text should be converted
			assert.equal(blocks.length, 1);
			assert.equal(blocks[0].type, "text");
		});
	});
});
