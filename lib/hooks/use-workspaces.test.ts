import assert from "node:assert";
import { describe, it } from "node:test";
import type { ProvidersResponse } from "../api-types";

// Test the logic patterns used in the useSandboxProviders hook
// Since we can't easily mock SWR hooks in Node's test runner,
// we test the core data transformation logic separately

describe("useSandboxProviders hook logic", () => {
	// Helper function that simulates the hook's data transformation logic
	const extractProviderData = (data: ProvidersResponse | undefined) => ({
		providers: data?.providers ? Object.keys(data.providers) : [],
		providerStatuses: data?.providers || {},
		defaultProvider: data?.default || "",
	});

	describe("provider extraction from API response", () => {
		it("should extract provider names from valid response", () => {
			const data: ProvidersResponse = {
				providers: {
					docker: { available: true, state: "ready" },
					vz: { available: true, state: "ready" },
				},
				default: "vz",
			};

			const result = extractProviderData(data);

			assert.deepStrictEqual(
				result.providers,
				["docker", "vz"],
				"Should extract provider names as array",
			);
			assert.strictEqual(result.providers.length, 2, "Should have 2 providers");
			assert.deepStrictEqual(
				result.providerStatuses,
				data.providers,
				"Should return provider statuses object",
			);
			assert.strictEqual(
				result.defaultProvider,
				"vz",
				"Should extract default provider",
			);
		});

		it("should handle undefined data gracefully", () => {
			const result = extractProviderData(undefined);

			assert.deepStrictEqual(
				result.providers,
				[],
				"Should return empty array when data is undefined",
			);
			assert.deepStrictEqual(
				result.providerStatuses,
				{},
				"Should return empty object for provider statuses",
			);
			assert.strictEqual(
				result.defaultProvider,
				"",
				"Should return empty string for default provider",
			);
		});

		it("should handle missing providers field gracefully", () => {
			// This tests the bug fix where data exists but providers is undefined
			const data = {
				providers: undefined,
				default: "vz",
			} as unknown as ProvidersResponse | undefined;

			const result = extractProviderData(data);

			assert.deepStrictEqual(
				result.providers,
				[],
				"Should return empty array when providers field is undefined",
			);
			assert.deepStrictEqual(
				result.providerStatuses,
				{},
				"Should return empty object when providers is undefined",
			);
			assert.strictEqual(
				result.defaultProvider,
				"vz",
				"Should still extract default provider",
			);
		});

		it("should handle empty providers object", () => {
			const data: ProvidersResponse = {
				providers: {},
				default: "",
			};

			const result = extractProviderData(data);

			assert.deepStrictEqual(
				result.providers,
				[],
				"Should return empty array for empty providers object",
			);
			assert.deepStrictEqual(
				result.providerStatuses,
				{},
				"Should return empty object",
			);
			assert.strictEqual(
				result.defaultProvider,
				"",
				"Should return empty default",
			);
		});

		it("should handle single provider", () => {
			const data: ProvidersResponse = {
				providers: {
					docker: { available: true, state: "ready" },
				},
				default: "docker",
			};

			const result = extractProviderData(data);

			assert.deepStrictEqual(
				result.providers,
				["docker"],
				"Should handle single provider",
			);
			assert.strictEqual(result.providers.length, 1, "Should have 1 provider");
		});

		it("should handle multiple providers in different states", () => {
			const data: ProvidersResponse = {
				providers: {
					docker: { available: true, state: "ready" },
					vz: {
						available: true,
						state: "downloading",
						message: "Downloading images...",
					},
					local: {
						available: false,
						state: "not_available",
						message: "Not enabled",
					},
				},
				default: "docker",
			};

			const result = extractProviderData(data);

			assert.deepStrictEqual(
				result.providers,
				["docker", "vz", "local"],
				"Should extract all provider names regardless of state",
			);
			assert.strictEqual(result.providers.length, 3, "Should have 3 providers");
			assert.strictEqual(
				result.providerStatuses.vz.state,
				"downloading",
				"Should preserve provider state",
			);
			assert.strictEqual(
				result.providerStatuses.local.available,
				false,
				"Should preserve availability flag",
			);
		});
	});

	describe("hasMultipleProviders logic", () => {
		it("should return true for two providers", () => {
			const providers = ["docker", "vz"];
			const hasMultipleProviders = providers.length > 1;

			assert.strictEqual(
				hasMultipleProviders,
				true,
				"Should return true for 2 providers",
			);
		});

		it("should return false for one provider", () => {
			const providers = ["docker"];
			const hasMultipleProviders = providers.length > 1;

			assert.strictEqual(
				hasMultipleProviders,
				false,
				"Should return false for 1 provider",
			);
		});

		it("should return false for no providers", () => {
			const providers: string[] = [];
			const hasMultipleProviders = providers.length > 1;

			assert.strictEqual(
				hasMultipleProviders,
				false,
				"Should return false for no providers",
			);
		});

		it("should return true for three or more providers", () => {
			const providers = ["docker", "vz", "local"];
			const hasMultipleProviders = providers.length > 1;

			assert.strictEqual(
				hasMultipleProviders,
				true,
				"Should return true for 3+ providers",
			);
		});
	});

	describe("provider response format compatibility", () => {
		it("should match expected ProvidersResponse shape", () => {
			const response: ProvidersResponse = {
				providers: {
					docker: {
						available: true,
						state: "ready",
					},
				},
				default: "docker",
			};

			// Type assertions - these verify the interface matches expectations
			assert.strictEqual(typeof response.providers, "object");
			assert.strictEqual(typeof response.default, "string");
			assert.strictEqual(typeof response.providers.docker.available, "boolean");
			assert.strictEqual(typeof response.providers.docker.state, "string");
		});

		it("should support optional provider status fields", () => {
			const response: ProvidersResponse = {
				providers: {
					vz: {
						available: true,
						state: "downloading",
						message: "Downloading kernel...",
						details: { progress: 0.5 },
					},
				},
				default: "vz",
			};

			const status = response.providers.vz;
			assert.strictEqual(status.message, "Downloading kernel...");
			assert.ok(status.details);
		});
	});
});

describe("Workspace hook integration patterns", () => {
	it("should coordinate workspace and provider data", () => {
		// Simulate scenario where workspace form needs provider list
		const providersData: ProvidersResponse = {
			providers: {
				docker: { available: true, state: "ready" },
				vz: { available: true, state: "ready" },
			},
			default: "vz",
		};

		const providers = providersData?.providers
			? Object.keys(providersData.providers)
			: [];
		const hasMultipleProviders = providers.length > 1;

		// When creating a workspace, the provider selection should only show if multiple providers
		assert.strictEqual(
			hasMultipleProviders,
			true,
			"Should show provider selection when multiple available",
		);

		// Selected provider should be from available list or undefined for default
		const selectedProvider = undefined; // undefined means use default
		const isValidSelection =
			selectedProvider === undefined || providers.includes(selectedProvider);

		assert.strictEqual(
			isValidSelection,
			true,
			"undefined is valid (uses default)",
		);
	});

	it("should handle workspace creation request with optional provider", () => {
		// Test the pattern where provider is only included if explicitly set
		const createWorkspaceRequest = (
			path: string,
			sourceType: "local" | "git",
			provider?: string,
		) => {
			const request: { path: string; sourceType: string; provider?: string } = {
				path,
				sourceType,
			};
			if (provider !== undefined) {
				request.provider = provider;
			}
			return request;
		};

		// Case 1: No provider selected (use default)
		const req1 = createWorkspaceRequest("~/projects/app", "local", undefined);
		assert.strictEqual(req1.provider, undefined, "Should omit provider field");

		// Case 2: Provider explicitly selected
		const req2 = createWorkspaceRequest("~/projects/app", "local", "vz");
		assert.strictEqual(req2.provider, "vz", "Should include provider field");
	});
});
