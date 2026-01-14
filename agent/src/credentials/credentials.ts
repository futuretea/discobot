import { createHash } from "node:crypto";

// CredentialEnvVar matches the server's CredentialEnvVar type
// The server resolves the provider -> env var mapping, so we just use it directly
export interface CredentialEnvVar {
	envVar: string;
	value: string;
}

// In-memory credential storage
let currentCredentials: CredentialEnvVar[] = [];
let credentialsHash = "";

// Compute a hash of credentials for change detection
function computeCredentialsHash(creds: CredentialEnvVar[]): string {
	// Sort by envVar for consistent ordering
	const sorted = [...creds].sort((a, b) => a.envVar.localeCompare(b.envVar));
	const json = JSON.stringify(sorted);
	return createHash("sha256").update(json).digest("hex");
}

// Parse credentials from header value
export function parseCredentialsHeader(
	headerValue: string | null,
): CredentialEnvVar[] {
	if (!headerValue) {
		return [];
	}

	try {
		const parsed = JSON.parse(headerValue);
		if (!Array.isArray(parsed)) {
			console.warn("Credentials header is not an array");
			return [];
		}
		return parsed as CredentialEnvVar[];
	} catch (error) {
		console.warn("Failed to parse credentials header:", error);
		return [];
	}
}

// Update credentials and return true if they changed
export function updateCredentials(creds: CredentialEnvVar[]): boolean {
	const newHash = computeCredentialsHash(creds);

	if (newHash === credentialsHash) {
		return false;
	}

	currentCredentials = creds;
	credentialsHash = newHash;
	return true;
}

// Get current credentials
export function getCredentials(): CredentialEnvVar[] {
	return currentCredentials;
}

// Convert credentials to environment variables
// Simple: just map envVar -> value
export function credentialsToEnv(
	creds: CredentialEnvVar[],
): Record<string, string> {
	const env: Record<string, string> = {};

	for (const cred of creds) {
		if (cred.envVar && cred.value) {
			env[cred.envVar] = cred.value;
		}
	}

	return env;
}

// Check if credentials have changed and return the new environment if so
export function checkCredentialsChanged(headerValue: string | null): {
	changed: boolean;
	env: Record<string, string>;
} {
	const creds = parseCredentialsHeader(headerValue);
	const changed = updateCredentials(creds);
	const env = credentialsToEnv(creds);

	return { changed, env };
}
