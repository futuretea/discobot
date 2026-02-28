import * as Diff from "diff";

/**
 * Diff size thresholds (in lines)
 * These help prevent UI blocking on very large diffs
 */
export const DIFF_WARNING_THRESHOLD = 10000; // Show warning but allow loading
export const DIFF_HARD_LIMIT = 20000; // Never render, show fallback only

/**
 * Language mapping for Monaco Editor syntax highlighting
 * Maps file extensions to Monaco language identifiers
 */
const LANGUAGE_MAP: Record<string, string> = {
	js: "javascript",
	jsx: "javascript",
	ts: "typescript",
	tsx: "typescript",
	py: "python",
	rb: "ruby",
	go: "go",
	rs: "rust",
	java: "java",
	c: "c",
	cpp: "cpp",
	h: "c",
	hpp: "cpp",
	cs: "csharp",
	php: "php",
	swift: "swift",
	kt: "kotlin",
	scala: "scala",
	html: "html",
	htm: "html",
	css: "css",
	scss: "scss",
	less: "less",
	json: "json",
	xml: "xml",
	yaml: "yaml",
	yml: "yaml",
	md: "markdown",
	sql: "sql",
	sh: "shell",
	bash: "shell",
	zsh: "shell",
	ps1: "powershell",
	dockerfile: "dockerfile",
	makefile: "makefile",
	toml: "toml",
	ini: "ini",
	conf: "ini",
	graphql: "graphql",
	gql: "graphql",
	vue: "vue",
	svelte: "svelte",
};

/**
 * Detect the programming language from a file path for syntax highlighting
 */
export function getLanguageFromPath(filePath: string): string {
	const ext = filePath.split(".").pop()?.toLowerCase() || "";

	// Check for special filenames
	const filename = filePath.split("/").pop()?.toLowerCase() || "";
	if (filename === "dockerfile") return "dockerfile";
	if (filename === "makefile") return "makefile";
	if (filename.startsWith(".") && !ext) return "plaintext";

	return LANGUAGE_MAP[ext] || "plaintext";
}

/**
 * Reconstruct the original content from current content and a unified diff patch.
 * The patch format is: original -> modified
 * So we need to apply the patch in reverse to go from modified back to original.
 */
export function reconstructOriginalFromPatch(
	currentContent: string,
	patch: string,
): string {
	try {
		// Parse the patch to get the structured patch object
		const parsedPatches = Diff.parsePatch(patch);
		if (parsedPatches.length === 0) {
			return currentContent;
		}

		// The patch goes from old -> new, so we need to reverse it
		// Apply the patch in reverse by swapping additions and deletions
		const reversedPatch = parsedPatches[0];

		// Swap old and new for reverse application
		const originalPatch = {
			...reversedPatch,
			hunks: reversedPatch.hunks.map((hunk) => ({
				...hunk,
				lines: hunk.lines.map((line) => {
					// Swap + and - to reverse the patch
					if (line.startsWith("+")) {
						return `-${line.slice(1)}`;
					}
					if (line.startsWith("-")) {
						return `+${line.slice(1)}`;
					}
					return line;
				}),
				oldStart: hunk.newStart,
				oldLines: hunk.newLines,
				newStart: hunk.oldStart,
				newLines: hunk.oldLines,
			})),
		};

		// Apply the reversed patch to get the original content
		const result = Diff.applyPatch(currentContent, originalPatch);
		return typeof result === "string" ? result : currentContent;
	} catch (error) {
		console.error("Failed to reconstruct original from patch:", error);
		return currentContent;
	}
}

/**
 * A line range in the new (current) file that corresponds to added or modified
 * content, as computed from a unified diff patch.
 */
export interface PatchLineRange {
	/** 1-based start line in the new file */
	startLine: number;
	/** 1-based end line in the new file (inclusive) */
	endLine: number;
	/** added = the hunk has no deletions; modified = the hunk also removes lines */
	type: "added" | "modified";
}

/**
 * Parse a unified diff patch and return the line ranges in the new file that
 * correspond to additions/modifications.  Used to decorate the Monaco editor
 * in "edit" mode so the user can see which lines were changed without switching
 * to the full diff view.
 *
 * Algorithm: iterate over each hunk.  If the hunk contains any deletion lines
 * the additions are classified as "modified"; otherwise "added".  Consecutive
 * addition lines are merged into a single range; context lines flush the
 * current range.
 */
export function parsePatchDecorations(patch: string): PatchLineRange[] {
	const parsed = Diff.parsePatch(patch);
	if (parsed.length === 0) return [];

	const result: PatchLineRange[] = [];

	for (const hunk of parsed[0].hunks) {
		const type = hunk.lines.some((l) => l.startsWith("-"))
			? "modified"
			: "added";

		let newLineNum = hunk.newStart;
		let rangeStart: number | null = null;

		for (const line of hunk.lines) {
			if (line.startsWith("+")) {
				if (rangeStart === null) rangeStart = newLineNum;
				newLineNum++;
			} else if (line.startsWith("-")) {
				// Deletion: doesn't advance the new-file line counter
			} else {
				// Context line: flush accumulated addition range
				if (rangeStart !== null) {
					result.push({ startLine: rangeStart, endLine: newLineNum - 1, type });
					rangeStart = null;
				}
				newLineNum++;
			}
		}
		if (rangeStart !== null) {
			result.push({ startLine: rangeStart, endLine: newLineNum - 1, type });
		}
	}

	return result;
}

/**
 * Fast count of diff lines without parsing the entire patch.
 * Counts lines that start with ' ', '+', or '-' (diff content lines).
 * This is much faster than parsing for large diffs.
 */
export function countDiffLinesFast(patch: string): number {
	let count = 0;
	let inHunk = false;

	for (const line of patch.split("\n")) {
		// Start of a hunk
		if (line.startsWith("@@")) {
			inHunk = true;
			continue;
		}

		// Count actual diff content lines (context, additions, deletions)
		if (
			inHunk &&
			(line.startsWith(" ") || line.startsWith("+") || line.startsWith("-"))
		) {
			count++;
		}
	}

	return count;
}
