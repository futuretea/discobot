const isTauri = process.env.NEXT_PUBLIC_TAURI === "true";
const isDev = process.env.NODE_ENV === "development";
const isTauriBuild = isTauri && !isDev;

/** @type {import('next').NextConfig} */
const nextConfig = {
	typescript: {
		ignoreBuildErrors: true,
	},
	images: {
		unoptimized: true,
	},
	// Use separate build directories for Tauri vs regular dev
	...(isTauriBuild && {
		output: "export",
		distDir: "dist",
	}),
	...(isTauri &&
		isDev && {
			distDir: ".next-tauri",
		}),
	// Rewrites work in server mode and Tauri dev mode (not static export)
	...(!isTauriBuild && {
		async rewrites() {
			return [
				// Proxy API requests to Go backend
				{
					source: "/api/:path*",
					destination: "http://localhost:3001/api/:path*",
				},
				// Proxy auth requests to Go backend
				{
					source: "/auth/:path*",
					destination: "http://localhost:3001/auth/:path*",
				},
				// Health check
				{
					source: "/health",
					destination: "http://localhost:3001/health",
				},
			];
		},
	}),
};

export default nextConfig;
