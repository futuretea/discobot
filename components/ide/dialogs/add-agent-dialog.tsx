import { Check, CheckCircle2, ChevronDown, Key, Search } from "lucide-react";

import * as React from "react";
import { IconRenderer } from "@/components/ide/icon-renderer";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type {
	Agent,
	AuthProvider,
	CreateAgentRequest,
	SupportedAgentType,
} from "@/lib/api-types";
import { useAgentTypes } from "@/lib/hooks/use-agent-types";
import {
	getAuthProviderLogoUrl,
	useAuthProviders,
} from "@/lib/hooks/use-auth-providers";
import { useCredentials } from "@/lib/hooks/use-credentials";
import { cn } from "@/lib/utils";

function ProviderLogo({
	providerId,
	className,
}: {
	providerId: string;
	className?: string;
}) {
	const [hasError, setHasError] = React.useState(false);
	const logoUrl = getAuthProviderLogoUrl(providerId);

	if (hasError) {
		return <Key className={className} />;
	}

	return (
		<img
			src={logoUrl}
			alt=""
			width={24}
			height={24}
			className={cn("object-contain dark:invert", className)}
			onError={() => setHasError(true)}
		/>
	);
}

interface AuthProviderRowProps {
	providerId: string;
	provider?: AuthProvider;
	isConfigured: boolean;
	onConfigure: () => void;
}

function AuthProviderRow({
	providerId,
	provider,
	isConfigured,
	onConfigure,
}: AuthProviderRowProps) {
	const displayName = provider?.name ?? providerId;

	return (
		<div className="flex items-center justify-between gap-3 py-2 px-3 rounded-lg bg-muted/30 border">
			<div className="flex items-center gap-3 min-w-0">
				<div className="h-5 w-5 rounded flex items-center justify-center shrink-0 overflow-hidden bg-background">
					<ProviderLogo providerId={providerId} className="h-4 w-4" />
				</div>
				<div className="min-w-0">
					<div className="text-sm font-medium truncate">{displayName}</div>
				</div>
			</div>
			<div className="flex items-center gap-2 shrink-0">
				{isConfigured ? (
					<div className="flex items-center gap-1.5 text-xs text-green-600 dark:text-green-500">
						<CheckCircle2 className="h-3.5 w-3.5" />
						Configured
					</div>
				) : (
					<Button
						variant="outline"
						size="sm"
						className="h-7 text-xs"
						onClick={onConfigure}
					>
						Configure
					</Button>
				)}
			</div>
		</div>
	);
}

// Component to show auth providers with optional search
function AuthProvidersSection({
	selectedType,
	providers,
	providersMap,
	configuredProviderIds,
	onOpenCredentials,
}: {
	selectedType: SupportedAgentType | null;
	providers: AuthProvider[];
	providersMap: Record<string, AuthProvider>;
	configuredProviderIds: Set<string>;
	onOpenCredentials?: (providerId?: string) => void;
}) {
	const [search, setSearch] = React.useState("");

	// Reset search when selected type changes
	// biome-ignore lint/correctness/useExhaustiveDependencies: intentional - reset search when type changes
	React.useEffect(() => {
		setSearch("");
	}, [selectedType?.id]);

	if (
		!selectedType?.supportedAuthProviders ||
		selectedType.supportedAuthProviders.length === 0
	) {
		return null;
	}

	// Handle '*' wildcard - show all providers with env vars
	const isWildcard = selectedType.supportedAuthProviders.includes("*");
	const allProviderIds = isWildcard
		? providers.filter((p) => p.env && p.env.length > 0).map((p) => p.id)
		: selectedType.supportedAuthProviders;

	const showSearch = allProviderIds.length > 6;

	// Filter providers by search
	const filteredProviderIds =
		showSearch && search.trim()
			? allProviderIds.filter((id) => {
					const provider = providersMap[id];
					const query = search.toLowerCase();
					return (
						id.toLowerCase().includes(query) ||
						provider?.name?.toLowerCase().includes(query) ||
						provider?.env?.some((e) => e.toLowerCase().includes(query))
					);
				})
			: allProviderIds;

	return (
		<div className="space-y-3">
			<div className="flex items-center justify-between">
				<Label>Authentication</Label>
				{onOpenCredentials && (
					<Button
						variant="ghost"
						size="sm"
						className="h-7 text-xs gap-1.5"
						onClick={() => {
							onOpenCredentials();
						}}
					>
						<Key className="h-3 w-3" />
						Manage All
					</Button>
				)}
			</div>

			{showSearch && (
				<div className="relative">
					<Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
					<Input
						value={search}
						onChange={(e) => setSearch(e.target.value)}
						placeholder="Search providers..."
						className="pl-9 h-8"
					/>
				</div>
			)}

			<div className="space-y-2 max-h-[200px] overflow-y-auto">
				{filteredProviderIds.length === 0 ? (
					<div className="py-4 text-center text-sm text-muted-foreground">
						No providers found
					</div>
				) : (
					filteredProviderIds.map((providerId) => (
						<AuthProviderRow
							key={providerId}
							providerId={providerId}
							provider={providersMap[providerId]}
							isConfigured={configuredProviderIds.has(providerId)}
							onConfigure={() => {
								if (onOpenCredentials) {
									onOpenCredentials(providerId);
								}
							}}
						/>
					))
				)}
			</div>
			<p className="text-xs text-muted-foreground">
				Configure at least one provider to use this agent.
			</p>
		</div>
	);
}

interface AddAgentDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onAdd: (agent: CreateAgentRequest) => Promise<void>;
	editingAgent?: Agent | null;
	onOpenCredentials?: (providerId?: string) => void;
	preselectedAgentTypeId?: string | null;
}

export function AddAgentDialog({
	open,
	onOpenChange,
	onAdd,
	editingAgent,
	onOpenCredentials,
	preselectedAgentTypeId,
}: AddAgentDialogProps) {
	const { agentTypes, isLoading } = useAgentTypes();
	const { credentials } = useCredentials();
	const { providers, providersMap } = useAuthProviders();
	const [selectedType, setSelectedType] =
		React.useState<SupportedAgentType | null>(null);
	const [isSubmitting, setIsSubmitting] = React.useState(false);

	// Auto-select agent type when preselectedAgentTypeId is provided
	React.useEffect(() => {
		if (preselectedAgentTypeId && agentTypes.length > 0 && !selectedType) {
			const agentType = agentTypes.find((t) => t.id === preselectedAgentTypeId);
			if (agentType) {
				setSelectedType(agentType);
			}
		}
	}, [preselectedAgentTypeId, agentTypes, selectedType]);

	// Get configured provider IDs
	const configuredProviderIds = React.useMemo(
		() =>
			new Set(credentials.filter((c) => c.isConfigured).map((c) => c.provider)),
		[credentials],
	);

	// Check if at least one supported provider is configured
	const hasConfiguredProvider = React.useMemo(() => {
		if (!selectedType?.supportedAuthProviders?.length) return true; // No auth required

		const isWildcard = selectedType.supportedAuthProviders.includes("*");
		const supportedIds = isWildcard
			? providers.filter((p) => p.env && p.env.length > 0).map((p) => p.id)
			: selectedType.supportedAuthProviders;

		return supportedIds.some((id) => configuredProviderIds.has(id));
	}, [selectedType, providers, configuredProviderIds]);

	React.useEffect(() => {
		if (editingAgent && agentTypes.length > 0) {
			const type = agentTypes.find((t) => t.id === editingAgent.agentType);
			if (type) setSelectedType(type);
		}
	}, [editingAgent, agentTypes]);

	const handleReset = () => {
		setSelectedType(null);
	};

	const handleOpenChange = (newOpen: boolean) => {
		if (!newOpen) {
			handleReset();
		}
		onOpenChange(newOpen);
	};

	const handleSubmit = async () => {
		if (!selectedType) return;

		setIsSubmitting(true);
		try {
			await onAdd({
				agentType: selectedType.id,
			});
			handleReset();
			onOpenChange(false);
		} finally {
			setIsSubmitting(false);
		}
	};

	const isEditing = !!editingAgent;
	const dialogTitle = isEditing ? "Configure Agent" : "Add Agent";
	const dialogDescription = isEditing
		? "Update this agent's configuration and capabilities."
		: "Create a new AI coding agent by selecting a type and configuring its capabilities.";
	const submitButtonText = isEditing
		? isSubmitting
			? "Saving..."
			: "Save Changes"
		: isSubmitting
			? "Creating..."
			: "Create Agent";

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogContent className="sm:max-w-[550px] max-h-[85vh] overflow-hidden flex flex-col">
				<DialogHeader>
					<DialogTitle>{dialogTitle}</DialogTitle>
					<DialogDescription>{dialogDescription}</DialogDescription>
				</DialogHeader>

				<div className="flex-1 overflow-y-auto space-y-6 py-4 pr-2">
					<div className="space-y-2">
						<Label>Agent Type</Label>
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button
									variant="outline"
									className="w-full justify-between bg-transparent"
									disabled={isLoading}
								>
									{selectedType ? (
										<div className="flex items-center gap-2">
											<IconRenderer
												icons={selectedType.icons}
												className="h-4 w-4"
											/>
											<span>{selectedType.name}</span>
										</div>
									) : (
										<span className="text-muted-foreground">
											{isLoading ? "Loading..." : "Select agent type"}
										</span>
									)}
									<ChevronDown className="h-4 w-4 opacity-50" />
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent className="w-[500px]" align="start">
								{agentTypes.map((type) => (
									<DropdownMenuItem
										key={type.id}
										onClick={() => setSelectedType(type)}
										className="flex items-start gap-3 py-3"
									>
										<span className="h-5 w-5 mt-0.5 shrink-0 flex items-center justify-center">
											<IconRenderer icons={type.icons} className="h-5 w-5" />
										</span>
										<div className="flex-1 min-w-0">
											<div className="font-medium">{type.name}</div>
											<div className="text-xs text-muted-foreground line-clamp-2">
												{type.description}
											</div>
										</div>
										{selectedType?.id === type.id && (
											<Check className="h-4 w-4 text-primary shrink-0" />
										)}
									</DropdownMenuItem>
								))}
							</DropdownMenuContent>
						</DropdownMenu>
					</div>

					{/* Auth Providers Section */}
					<AuthProvidersSection
						selectedType={selectedType}
						providers={providers}
						providersMap={providersMap}
						configuredProviderIds={configuredProviderIds}
						onOpenCredentials={onOpenCredentials}
					/>
				</div>

				<DialogFooter className="border-t pt-4">
					<Button variant="outline" onClick={() => handleOpenChange(false)}>
						Cancel
					</Button>
					<Button
						onClick={handleSubmit}
						disabled={!selectedType || !hasConfiguredProvider || isSubmitting}
						title={
							!hasConfiguredProvider
								? "Configure at least one auth provider"
								: undefined
						}
					>
						{submitButtonText}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
