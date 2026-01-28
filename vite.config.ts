import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import tsconfigPaths from "vite-tsconfig-paths";

const isTauri = process.env.TAURI_ENV_PLATFORM !== undefined;

export default defineConfig({
	plugins: [
		react(),
		tailwindcss(),
		tsconfigPaths({
			// Only parse root tsconfig, ignore workspace directories
			projects: ["./tsconfig.json"],
		}),
	],
	define: {
		// Prevent process.env errors in browser
		"process.env.NODE_ENV": JSON.stringify(
			process.env.NODE_ENV || "development",
		),
	},
	server: {
		port: 3000,
		strictPort: true,
	},
	// Tauri expects a fixed port
	preview: {
		port: 3000,
		strictPort: true,
	},
	build: {
		outDir: isTauri ? "dist" : "out",
		emptyOutDir: true,
		// Optimize chunks
		rollupOptions: {
			output: {
				manualChunks: {
					monaco: ["@monaco-editor/react"],
					xterm: ["@xterm/xterm", "@xterm/addon-fit", "@xterm/addon-web-links"],
					"ai-sdk": ["ai", "@ai-sdk/react"],
					radix: [
						"@radix-ui/react-dialog",
						"@radix-ui/react-dropdown-menu",
						"@radix-ui/react-select",
						"@radix-ui/react-tabs",
						"@radix-ui/react-toast",
						"@radix-ui/react-tooltip",
						"@radix-ui/react-accordion",
					],
				},
			},
		},
		// Increase chunk size warning limit (Monaco is large)
		chunkSizeWarningLimit: 1000,
	},
	// Handle SSE streaming properly
	optimizeDeps: {
		exclude: ["@tauri-apps/api", "@tauri-apps/plugin-shell"],
	},
	// Clear screen disabled for better logging during dev
	clearScreen: false,
	// Tauri needs this for its custom protocol
	envPrefix: ["VITE_", "TAURI_", "NEXT_PUBLIC_"],
});
