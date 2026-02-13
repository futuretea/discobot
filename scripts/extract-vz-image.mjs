#!/usr/bin/env node
/**
 * Extract VZ image files from Docker registry for Tauri bundling
 *
 * This script pulls the VZ Docker image from the registry and extracts
 * the kernel and rootfs files to src-tauri/resources/ for bundling into
 * the macOS app.
 *
 * Usage: node scripts/extract-vz-image.mjs <image-ref> [arch]
 *   image-ref: Docker image reference (e.g., ghcr.io/obot-platform/discobot-vz:v0.1.0)
 *   arch: Architecture (amd64 or arm64, defaults to host arch)
 */

import { execSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");
const resourcesDir = join(projectRoot, "src-tauri", "resources");

// Parse arguments
const imageRef = process.argv[2];
const arch = process.argv[3] || process.arch === "arm64" ? "arm64" : "amd64";

if (!imageRef) {
	console.error("Error: Image reference is required");
	console.error("Usage: node scripts/extract-vz-image.mjs <image-ref> [arch]");
	console.error(
		"Example: node scripts/extract-vz-image.mjs ghcr.io/obot-platform/discobot-vz:v0.1.0 arm64",
	);
	process.exit(1);
}

// Ensure resources directory exists
mkdirSync(resourcesDir, { recursive: true });

console.log(`Extracting VZ image files for ${arch}...`);
console.log(`Image: ${imageRef}`);
console.log(`Output directory: ${resourcesDir}`);

try {
	// Create a temporary container from the image with the specific architecture
	// Use a dummy command since the VZ image is FROM scratch and doesn't have a shell
	console.log(`Creating temporary container from ${imageRef}...`);
	const containerId = execSync(
		`docker create --platform linux/${arch} "${imageRef}" /bin/true`,
		{ encoding: "utf-8" },
	).trim();

	console.log(`Container created: ${containerId}`);

	try {
		// Extract the files
		const files = ["vmlinuz", "kernel-version", "discobot-rootfs.squashfs"];

		for (const file of files) {
			console.log(`Extracting ${file}...`);
			execSync(
				`docker cp "${containerId}:/${file}" "${resourcesDir}/${file}"`,
				{ stdio: "inherit" },
			);
		}

		console.log("VZ image files extracted successfully:");
		for (const file of files) {
			const filePath = join(resourcesDir, file);
			try {
				const stats = execSync(`ls -lh "${filePath}"`, { encoding: "utf-8" });
				console.log(`  ${stats.trim()}`);
			} catch {
				console.log(`  ${file} (size unknown)`);
			}
		}
	} finally {
		// Clean up: remove the temporary container
		console.log(`Removing temporary container ${containerId}...`);
		execSync(`docker rm "${containerId}"`, { stdio: "ignore" });
	}
} catch (error) {
	console.error("Failed to extract VZ image:", error.message);
	process.exit(1);
}
