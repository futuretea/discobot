import { Copy, ExternalLink, Key, Loader2, LogIn } from "lucide-react";

import * as React from "react";
import { mutate } from "swr";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api-client";
import { openUrl } from "@/lib/tauri";
import type {
	AuthPlugin,
	OAuthCompleteResult,
	OAuthOption,
	OAuthStartResult,
} from "./types";

/**
 * GitHub OAuth authentication plugin
 *
 * Uses GitHub's device authorization flow with repo + read:user + user:email scopes
 * so that discobot can clone private repos, push branches, and create PRs on behalf
 * of the user.
 */

const OAUTH_OPTIONS: OAuthOption[] = [
	{
		id: "github.com",
		label: "GitHub.com",
		description: "Sign in with GitHub",
		icon: "login",
	},
];

async function startOAuth(_optionId: string): Promise<OAuthStartResult> {
	// Handled entirely in the UI component
	return { url: "", verifier: "" };
}

async function completeOAuth(
	_code: string,
	_verifier: string,
): Promise<OAuthCompleteResult> {
	mutate("credentials");
	return { success: true };
}

/**
 * Provider logo component
 */
function ProviderLogo({ className }: { className?: string }) {
	const [hasError, setHasError] = React.useState(false);

	if (hasError) {
		return <Key className={className} />;
	}

	return (
		<img
			src="https://cdn.simpleicons.org/github"
			alt=""
			width={24}
			height={24}
			className={`${className} dark:invert`}
			style={{ objectFit: "contain" }}
			onError={() => setHasError(true)}
		/>
	);
}

interface GitHubOAuthFlowProps {
	onComplete: () => void;
	onCancel: () => void;
}

type FlowStep = "connect" | "device-code" | "polling";

/**
 * GitHub OAuth flow UI component
 *
 * Steps:
 * 1. Connect button — user initiates the flow
 * 2. Show device code — user copies code and opens verification URL
 * 3. Polling — waits for GitHub to confirm authorization
 */
export function GitHubOAuthFlow({
	onComplete,
	onCancel,
}: GitHubOAuthFlowProps) {
	const [step, setStep] = React.useState<FlowStep>("connect");
	const [deviceInfo, setDeviceInfo] = React.useState<{
		verificationUri: string;
		userCode: string;
		deviceCode: string;
		domain: string;
		interval: number;
	} | null>(null);
	const [isLoading, setIsLoading] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const [copied, setCopied] = React.useState(false);
	const pollingRef = React.useRef<NodeJS.Timeout | null>(null);

	React.useEffect(() => {
		return () => {
			if (pollingRef.current) {
				clearTimeout(pollingRef.current);
			}
		};
	}, []);

	const startDeviceFlow = async () => {
		setIsLoading(true);
		setError(null);

		try {
			const result = await api.githubDeviceCode();

			setDeviceInfo({
				verificationUri: result.verificationUri,
				userCode: result.userCode,
				deviceCode: result.deviceCode,
				domain: result.domain,
				interval: result.interval,
			});
			setStep("device-code");

			openUrl(result.verificationUri);
		} catch (err) {
			setError(
				err instanceof Error ? err.message : "Failed to start device flow",
			);
		} finally {
			setIsLoading(false);
		}
	};

	const startPolling = () => {
		if (!deviceInfo) return;

		setStep("polling");
		setError(null);

		const poll = async () => {
			try {
				const result = await api.githubPoll({
					deviceCode: deviceInfo.deviceCode,
					domain: deviceInfo.domain,
				});

				if (result.status === "success") {
					mutate("credentials");
					onComplete();
				} else if (result.status === "pending") {
					pollingRef.current = setTimeout(poll, deviceInfo.interval * 1000);
				} else {
					setError(result.error || "Authorization failed");
					setStep("device-code");
				}
			} catch (err) {
				setError(err instanceof Error ? err.message : "Polling failed");
				setStep("device-code");
			}
		};

		poll();
	};

	const handleCopyCode = () => {
		if (deviceInfo?.userCode) {
			navigator.clipboard.writeText(deviceInfo.userCode);
			setCopied(true);
			setTimeout(() => setCopied(false), 2000);
		}
	};

	// Step 1: Connect button
	if (step === "connect") {
		return (
			<div className="space-y-4">
				<div className="flex items-center gap-3 pb-3 border-b">
					<div className="h-8 w-8 rounded-md flex items-center justify-center bg-muted overflow-hidden">
						<ProviderLogo className="h-5 w-5" />
					</div>
					<div>
						<div className="font-medium">GitHub</div>
						<div className="text-xs text-muted-foreground">
							Connect your GitHub account
						</div>
					</div>
				</div>

				<p className="text-sm text-muted-foreground">
					Authorize discobot to clone repositories, push branches, and create
					pull requests on your behalf.
				</p>

				{error && <p className="text-sm text-destructive">{error}</p>}

				<div className="flex justify-end gap-2">
					<Button variant="ghost" size="sm" onClick={onCancel}>
						Cancel
					</Button>
					<Button
						size="sm"
						className="gap-2"
						onClick={startDeviceFlow}
						disabled={isLoading}
					>
						{isLoading ? (
							<>
								<Loader2 className="h-4 w-4 animate-spin" />
								Connecting...
							</>
						) : (
							<>
								<LogIn className="h-4 w-4" />
								Connect with GitHub
							</>
						)}
					</Button>
				</div>
			</div>
		);
	}

	// Step 2: Show device code
	if (step === "device-code" && deviceInfo) {
		return (
			<div className="space-y-4">
				<div className="flex items-center gap-3 pb-3 border-b">
					<div className="h-8 w-8 rounded-md flex items-center justify-center bg-muted overflow-hidden">
						<ProviderLogo className="h-5 w-5" />
					</div>
					<div>
						<div className="font-medium">GitHub</div>
						<div className="text-xs text-muted-foreground">
							Enter the code on GitHub
						</div>
					</div>
				</div>

				<div className="space-y-3">
					<p className="text-sm text-muted-foreground">
						A browser window should have opened. Enter this code on GitHub to
						authorize:
					</p>

					<div className="flex items-center gap-2">
						<div className="flex-1 bg-muted rounded-lg p-4 text-center">
							<code className="text-2xl font-bold tracking-widest">
								{deviceInfo.userCode}
							</code>
						</div>
						<Button
							variant="outline"
							size="icon"
							className="h-14 w-14"
							onClick={handleCopyCode}
						>
							<Copy className="h-5 w-5" />
						</Button>
					</div>

					{copied && (
						<p className="text-xs text-center text-muted-foreground">
							Copied to clipboard!
						</p>
					)}

					<Button
						variant="outline"
						size="sm"
						className="w-full gap-2"
						onClick={() => openUrl(deviceInfo.verificationUri)}
					>
						<ExternalLink className="h-3.5 w-3.5" />
						Open {deviceInfo.verificationUri}
					</Button>
				</div>

				{error && <p className="text-sm text-destructive">{error}</p>}

				<div className="flex justify-end gap-2">
					<Button variant="outline" size="sm" onClick={onCancel}>
						Cancel
					</Button>
					<Button size="sm" onClick={startPolling}>
						I've Entered the Code
					</Button>
				</div>
			</div>
		);
	}

	// Step 3: Polling
	if (step === "polling") {
		return (
			<div className="space-y-4">
				<div className="flex items-center gap-3 pb-3 border-b">
					<div className="h-8 w-8 rounded-md flex items-center justify-center bg-muted overflow-hidden">
						<ProviderLogo className="h-5 w-5" />
					</div>
					<div>
						<div className="font-medium">GitHub</div>
						<div className="text-xs text-muted-foreground">
							Waiting for authorization...
						</div>
					</div>
				</div>

				<div className="flex flex-col items-center gap-4 py-6">
					<Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
					<p className="text-sm text-muted-foreground text-center">
						Waiting for you to authorize on GitHub...
						<br />
						This will complete automatically.
					</p>
				</div>

				{error && <p className="text-sm text-destructive">{error}</p>}

				<div className="flex justify-end">
					<Button
						variant="outline"
						size="sm"
						onClick={() => {
							if (pollingRef.current) {
								clearTimeout(pollingRef.current);
							}
							onCancel();
						}}
					>
						Cancel
					</Button>
				</div>
			</div>
		);
	}

	return null;
}

/**
 * GitHub auth plugin implementation
 */
export const githubAuthPlugin: AuthPlugin = {
	providerId: "github-git",
	label: "Sign in with GitHub",
	oauthOptions: OAUTH_OPTIONS,
	oauthFlow: GitHubOAuthFlow,
	startOAuth,
	completeOAuth,
};

export default githubAuthPlugin;
