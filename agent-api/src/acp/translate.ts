import type { ContentBlock } from "@agentclientprotocol/sdk";
import type { UIMessage } from "ai";

/**
 * Convert UIMessage to ACP ContentBlock array.
 * This is the core translation function for sending prompts to ACP agents.
 */
export function uiMessageToContentBlocks(message: UIMessage): ContentBlock[] {
	const blocks: ContentBlock[] = [];

	for (const part of message.parts) {
		if (part.type === "text") {
			blocks.push({
				type: "text",
				text: part.text,
			});
		} else if (part.type === "file") {
			blocks.push({
				type: "resource_link",
				uri: part.url,
				name: part.filename || "file",
				mimeType: part.mediaType,
			});
		}
	}

	return blocks;
}
