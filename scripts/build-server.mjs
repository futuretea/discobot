import { execSync } from "child_process";
import { mkdirSync } from "fs";
import { dirname, join } from "path";
import { fileURLToPath } from "url";
import os from "os";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(__dirname);
const serverDir = join(projectRoot, "server");
const binariesDir = join(projectRoot, "src-tauri", "binaries");

// Create binaries directory
mkdirSync(binariesDir, { recursive: true });

// Get target triple for current platform
function getTargetTriple() {
  const platform = os.platform();
  const arch = os.arch();

  if (platform === "linux") {
    if (arch === "x64") return "x86_64-unknown-linux-gnu";
    if (arch === "arm64") return "aarch64-unknown-linux-gnu";
  } else if (platform === "darwin") {
    if (arch === "x64") return "x86_64-apple-darwin";
    if (arch === "arm64") return "aarch64-apple-darwin";
  } else if (platform === "win32") {
    if (arch === "x64") return "x86_64-pc-windows-msvc";
  }

  throw new Error(`Unsupported platform: ${platform} ${arch}`);
}

const targetTriple = getTargetTriple();
const ext = os.platform() === "win32" ? ".exe" : "";
const outputName = `octobot-server-${targetTriple}${ext}`;
const outputPath = join(binariesDir, outputName);

console.log(`Building octobot-server for ${targetTriple}...`);

execSync(`go build -o "${outputPath}" ./cmd/server`, {
  cwd: serverDir,
  stdio: "inherit",
});

console.log(`Built: ${outputPath}`);
