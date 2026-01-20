import assert from "node:assert/strict";
import { mkdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { after, before, describe, it } from "node:test";
import type {
	DiffResponse,
	ErrorResponse,
	ListFilesResponse,
	ReadFileResponse,
	WriteFileResponse,
} from "../../src/api/types.js";
import { createApp } from "../../src/server/app.js";

describe("File System API Endpoints", () => {
	const testDir = "/tmp/agent-api-integration-files";
	let app: ReturnType<typeof createApp>["app"];

	before(async () => {
		// Clean up any existing test directory
		await rm(testDir, { recursive: true, force: true });

		// Create test directory structure
		await mkdir(join(testDir, "src/components"), { recursive: true });
		await mkdir(join(testDir, "lib"), { recursive: true });
		await mkdir(join(testDir, ".hidden"), { recursive: true });

		// Create test files
		await writeFile(join(testDir, "package.json"), '{"name": "test-project"}');
		await writeFile(
			join(testDir, "README.md"),
			"# Test Project\n\nThis is a test.",
		);
		await writeFile(join(testDir, ".gitignore"), "node_modules\ndist");
		await writeFile(join(testDir, ".env"), "SECRET=test123");
		await writeFile(
			join(testDir, "src/index.ts"),
			'export const main = () => "hello";',
		);
		await writeFile(
			join(testDir, "src/components/Button.tsx"),
			"export const Button = () => <button>Click</button>;",
		);

		// Create a binary-like file
		await writeFile(
			join(testDir, "binary.bin"),
			Buffer.from([0x00, 0x01, 0x02, 0x03, 0xff]),
		);

		// Create app with test directory as workspace root
		const result = createApp({
			agentCommand: "true",
			agentArgs: [],
			agentCwd: testDir,
			enableLogging: false,
		});
		app = result.app;
	});

	after(async () => {
		await rm(testDir, { recursive: true, force: true });
	});

	// =========================================================================
	// GET /files - Directory Listing
	// =========================================================================

	describe("GET /files", () => {
		it("lists root directory", async () => {
			const res = await app.request("/files?path=.");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ListFilesResponse;
			assert.equal(body.path, ".");
			assert.ok(Array.isArray(body.entries));

			// Check for expected entries
			const names = body.entries.map((e) => e.name);
			assert.ok(names.includes("src"), "Should include src directory");
			assert.ok(names.includes("lib"), "Should include lib directory");
			assert.ok(names.includes("package.json"), "Should include package.json");
			assert.ok(names.includes("README.md"), "Should include README.md");
		});

		it("defaults to root when path not provided", async () => {
			const res = await app.request("/files");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ListFilesResponse;
			assert.equal(body.path, ".");
		});

		it("excludes hidden files by default", async () => {
			const res = await app.request("/files?path=.");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ListFilesResponse;
			const names = body.entries.map((e) => e.name);

			assert.ok(!names.includes(".gitignore"), "Should not include .gitignore");
			assert.ok(!names.includes(".env"), "Should not include .env");
			assert.ok(
				!names.includes(".hidden"),
				"Should not include .hidden directory",
			);
		});

		it("includes hidden files when hidden=true", async () => {
			const res = await app.request("/files?path=.&hidden=true");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ListFilesResponse;
			const names = body.entries.map((e) => e.name);

			assert.ok(names.includes(".gitignore"), "Should include .gitignore");
			assert.ok(names.includes(".env"), "Should include .env");
			assert.ok(names.includes(".hidden"), "Should include .hidden directory");
		});

		it("lists subdirectory", async () => {
			const res = await app.request("/files?path=src");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ListFilesResponse;
			assert.equal(body.path, "src");

			const names = body.entries.map((e) => e.name);
			assert.ok(
				names.includes("components"),
				"Should include components directory",
			);
			assert.ok(names.includes("index.ts"), "Should include index.ts");
		});

		it("lists nested subdirectory", async () => {
			const res = await app.request("/files?path=src/components");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ListFilesResponse;
			assert.equal(body.path, "src/components");

			const names = body.entries.map((e) => e.name);
			assert.ok(names.includes("Button.tsx"), "Should include Button.tsx");
		});

		it("sorts directories before files", async () => {
			const res = await app.request("/files?path=.");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ListFilesResponse;

			// Find indices
			const directories = body.entries.filter((e) => e.type === "directory");
			const files = body.entries.filter((e) => e.type === "file");

			if (directories.length > 0 && files.length > 0) {
				const lastDirIndex = body.entries.findIndex(
					(e) => e.name === directories[directories.length - 1].name,
				);
				const firstFileIndex = body.entries.findIndex(
					(e) => e.name === files[0].name,
				);
				assert.ok(
					lastDirIndex < firstFileIndex,
					"All directories should come before files",
				);
			}
		});

		it("includes file sizes for files", async () => {
			const res = await app.request("/files?path=.");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ListFilesResponse;
			const packageJson = body.entries.find((e) => e.name === "package.json");

			assert.ok(packageJson, "Should find package.json");
			assert.equal(packageJson.type, "file");
			assert.ok(typeof packageJson.size === "number", "Should have size");
			assert.ok(packageJson.size > 0, "Size should be positive");
		});

		it("does not include size for directories", async () => {
			const res = await app.request("/files?path=.");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ListFilesResponse;
			const srcDir = body.entries.find((e) => e.name === "src");

			assert.ok(srcDir, "Should find src directory");
			assert.equal(srcDir.type, "directory");
			assert.equal(srcDir.size, undefined, "Directories should not have size");
		});

		it("returns 400 for path traversal attempt", async () => {
			const res = await app.request("/files?path=../etc");
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "Invalid path");
		});

		it("returns 400 for nested path traversal attempt", async () => {
			const res = await app.request("/files?path=src/../../etc");
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "Invalid path");
		});

		it("returns 404 for non-existent directory", async () => {
			const res = await app.request("/files?path=nonexistent");
			assert.equal(res.status, 404);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "Directory not found");
		});

		it("returns 400 when path is a file", async () => {
			const res = await app.request("/files?path=package.json");
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "Not a directory");
		});
	});

	// =========================================================================
	// GET /files/read - Read File
	// =========================================================================

	describe("GET /files/read", () => {
		it("reads text file as utf8", async () => {
			const res = await app.request("/files/read?path=package.json");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ReadFileResponse;
			assert.equal(body.path, "package.json");
			assert.equal(body.encoding, "utf8");
			assert.ok(body.content.includes('"name": "test-project"'));
			assert.ok(body.size > 0);
		});

		it("reads TypeScript file", async () => {
			const res = await app.request("/files/read?path=src/index.ts");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ReadFileResponse;
			assert.equal(body.path, "src/index.ts");
			assert.equal(body.encoding, "utf8");
			assert.ok(body.content.includes("export const main"));
		});

		it("reads file from nested directory", async () => {
			const res = await app.request(
				"/files/read?path=src/components/Button.tsx",
			);
			assert.equal(res.status, 200);

			const body = (await res.json()) as ReadFileResponse;
			assert.equal(body.path, "src/components/Button.tsx");
			assert.ok(body.content.includes("Button"));
		});

		it("reads binary file as base64", async () => {
			const res = await app.request("/files/read?path=binary.bin");
			assert.equal(res.status, 200);

			const body = (await res.json()) as ReadFileResponse;
			assert.equal(body.path, "binary.bin");
			assert.equal(body.encoding, "base64");

			// Decode and verify content
			const decoded = Buffer.from(body.content, "base64");
			assert.deepEqual(decoded, Buffer.from([0x00, 0x01, 0x02, 0x03, 0xff]));
		});

		it("returns 400 when path parameter is missing", async () => {
			const res = await app.request("/files/read");
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "path query parameter required");
		});

		it("returns 400 for path traversal attempt", async () => {
			const res = await app.request("/files/read?path=../etc/passwd");
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "Invalid path");
		});

		it("returns 404 for non-existent file", async () => {
			const res = await app.request("/files/read?path=nonexistent.txt");
			assert.equal(res.status, 404);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "File not found");
		});

		it("returns 400 when path is a directory", async () => {
			const res = await app.request("/files/read?path=src");
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "Is a directory");
		});
	});

	// =========================================================================
	// POST /files/write - Write File
	// =========================================================================

	describe("POST /files/write", () => {
		it("writes new text file", async () => {
			const res = await app.request("/files/write", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					path: "new-file.txt",
					content: "Hello, world!",
				}),
			});
			assert.equal(res.status, 200);

			const body = (await res.json()) as WriteFileResponse;
			assert.equal(body.path, "new-file.txt");
			assert.equal(body.size, 13);

			// Verify file was written
			const readRes = await app.request("/files/read?path=new-file.txt");
			const readBody = (await readRes.json()) as ReadFileResponse;
			assert.equal(readBody.content, "Hello, world!");
		});

		it("writes file with base64 encoding", async () => {
			const binaryContent = Buffer.from([
				0x48, 0x65, 0x6c, 0x6c, 0x6f,
			]).toString("base64");

			const res = await app.request("/files/write", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					path: "base64-file.bin",
					content: binaryContent,
					encoding: "base64",
				}),
			});
			assert.equal(res.status, 200);

			const body = (await res.json()) as WriteFileResponse;
			assert.equal(body.path, "base64-file.bin");
			assert.equal(body.size, 5); // "Hello" is 5 bytes
		});

		it("creates parent directories automatically", async () => {
			const res = await app.request("/files/write", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					path: "deep/nested/path/file.txt",
					content: "Nested content",
				}),
			});
			assert.equal(res.status, 200);

			const body = (await res.json()) as WriteFileResponse;
			assert.equal(body.path, "deep/nested/path/file.txt");

			// Verify file was written
			const readRes = await app.request(
				"/files/read?path=deep/nested/path/file.txt",
			);
			assert.equal(readRes.status, 200);
		});

		it("overwrites existing file", async () => {
			// Write initial content
			await app.request("/files/write", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					path: "overwrite-test.txt",
					content: "Original content",
				}),
			});

			// Overwrite
			const res = await app.request("/files/write", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					path: "overwrite-test.txt",
					content: "Updated content",
				}),
			});
			assert.equal(res.status, 200);

			// Verify content was updated
			const readRes = await app.request("/files/read?path=overwrite-test.txt");
			const readBody = (await readRes.json()) as ReadFileResponse;
			assert.equal(readBody.content, "Updated content");
		});

		it("returns 400 when path is missing", async () => {
			const res = await app.request("/files/write", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					content: "Hello",
				}),
			});
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "path is required");
		});

		it("returns 400 when content is missing", async () => {
			const res = await app.request("/files/write", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					path: "test.txt",
				}),
			});
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "content is required");
		});

		it("returns 400 for path traversal attempt", async () => {
			const res = await app.request("/files/write", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					path: "../escape.txt",
					content: "Malicious content",
				}),
			});
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "Invalid path");
		});
	});

	// =========================================================================
	// GET /diff - Session Diff
	// =========================================================================

	describe("GET /diff", () => {
		it("returns diff response structure", async () => {
			const res = await app.request("/diff");
			assert.equal(res.status, 200);

			const body = (await res.json()) as DiffResponse;
			assert.ok(Array.isArray(body.files), "Should have files array");
			assert.ok(typeof body.stats === "object", "Should have stats object");
			assert.ok(typeof body.stats.filesChanged === "number");
			assert.ok(typeof body.stats.additions === "number");
			assert.ok(typeof body.stats.deletions === "number");
		});

		it("returns file list with format=files", async () => {
			const res = await app.request("/diff?format=files");
			assert.equal(res.status, 200);

			const body = await res.json();
			assert.ok(Array.isArray(body.files), "Should have files array");
			assert.ok(typeof body.stats === "object", "Should have stats object");
		});

		it("returns 400 for path traversal in single file diff", async () => {
			const res = await app.request("/diff?path=../etc/passwd");
			assert.equal(res.status, 400);

			const body = (await res.json()) as ErrorResponse;
			assert.equal(body.error, "Invalid path");
		});
	});
});
