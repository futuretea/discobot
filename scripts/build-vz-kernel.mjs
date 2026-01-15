#!/usr/bin/env node
/**
 * Build Linux kernel for macOS Virtualization.framework
 *
 * This script only builds the kernel on macOS (darwin) since it's only
 * useful there. On other platforms, it skips the build.
 *
 * Usage: node scripts/build-vz-kernel.mjs [--force]
 *   --force: Build even on non-macOS platforms (for CI cross-compilation)
 */

import { execSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");
const resourcesDir = join(projectRoot, "src-tauri", "resources");

const isDarwin = process.platform === "darwin";
const forceFlag = process.argv.includes("--force");

// Ensure resources directory exists
mkdirSync(resourcesDir, { recursive: true });

if (!isDarwin && !forceFlag) {
	console.log("Skipping VZ kernel build (not on macOS)");
	console.log("Use --force to build anyway");
	process.exit(0);
}

// Check if Docker is available
try {
	execSync("docker --version", { stdio: "ignore" });
} catch {
	console.error("Error: Docker is not available");
	console.error("Docker is required to build the VZ kernel");
	process.exit(1);
}

console.log("Building VZ Linux kernel (this may take 10-20 minutes)...");
console.log(`Output directory: ${resourcesDir}`);

try {
	execSync(
		`docker build --target vz-kernel --output type=local,dest="${resourcesDir}" .`,
		{
			cwd: join(projectRoot, "agent"),
			stdio: "inherit",
		},
	);
	console.log("VZ kernel built successfully");

	// Verify output and show version
	const kernelFile = join(resourcesDir, "vmlinuz.zst");
	const versionFile = join(resourcesDir, "kernel-version");

	if (existsSync(kernelFile)) {
		console.log(`Output: ${kernelFile}`);
		if (existsSync(versionFile)) {
			const version = readFileSync(versionFile, "utf-8").trim();
			console.log(`Kernel version: ${version}`);
		}
	} else {
		console.error("Warning: Expected output file not found");
	}
} catch (error) {
	console.error("Failed to build VZ kernel:", error.message);
	process.exit(1);
}
