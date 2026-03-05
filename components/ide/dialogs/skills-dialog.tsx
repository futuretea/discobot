import { useMemo, useState } from "react";

import { BookOpen, Globe2, Loader2, Plus, RefreshCw, Search, X } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { SkillInUseError, api } from "@/lib/api-client";
import type { Skill, SkillMarketEntry, SkillMarketRepo } from "@/lib/api-types";
import { useSkillMarket, useSkillMarketRepos, useSkills } from "@/lib/hooks/use-skills";
import { cn } from "@/lib/utils";

interface SkillsDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
}

// ── Add-Repo inline form state ────────────────────────────────────────────
interface AddRepoFormState {
	name: string;
	repoUrl: string;
	branch: string;
	path: string;
}

const EMPTY_ADD_REPO_FORM: AddRepoFormState = { name: "", repoUrl: "", branch: "", path: "" };

export function SkillsDialog({ open, onOpenChange }: SkillsDialogProps) {
	const {
		skills,
		isLoading: isLoadingSkills,
		deleteSkill,
		importSkill,
	} = useSkills();

	// ── Repo management ───────────────────────────────────────────────────────
	const { repos, isLoading: isLoadingRepos, createRepo, deleteRepo } = useSkillMarketRepos();

	// ── Installed tab state ───────────────────────────────────────────────────
	const [activeTab, setActiveTab] = useState<"installed" | "market">("installed");
	const [selectedSkillId, setSelectedSkillId] = useState<string | null>(null);
	const [isDeleting, setIsDeleting] = useState(false);
	const [deleteError, setDeleteError] = useState<string | null>(null);
	const [search, setSearch] = useState("");

	// ── Market tab state ──────────────────────────────────────────────────────
	const [marketSearch, setMarketSearch] = useState("");
	const [activeRepoId, setActiveRepoId] = useState<string | null>(null);
	const [selectedMarketEntry, setSelectedMarketEntry] = useState<SkillMarketEntry | null>(null);
	const [installingId, setInstallingId] = useState<string | null>(null);
	const [installError, setInstallError] = useState<string | null>(null);
	const [isReloading, setIsReloading] = useState(false);

	// Add-repo form
	const [showAddRepoForm, setShowAddRepoForm] = useState(false);
	const [addRepoForm, setAddRepoForm] = useState<AddRepoFormState>(EMPTY_ADD_REPO_FORM);
	const [isSavingRepo, setIsSavingRepo] = useState(false);
	const [addRepoError, setAddRepoError] = useState<string | null>(null);
	const [deletingRepoId, setDeletingRepoId] = useState<string | null>(null);

	// Resolve the active repo object
	const activeRepo: SkillMarketRepo | null = useMemo(() => {
		if (!repos.length) return null;
		return repos.find((r) => r.id === activeRepoId) ?? repos[0];
	}, [repos, activeRepoId]);

	const effectiveMarketUrl = activeRepo?.repoUrl || undefined;
	const effectiveMarketBranch = activeRepo?.branch || undefined;
	const effectiveMarketPath = activeRepo?.path || undefined;

	const {
		skills: marketSkills,
		isLoading: isLoadingMarket,
		error: marketError,
		reload: reloadMarket,
	} = useSkillMarket(effectiveMarketUrl, effectiveMarketBranch, effectiveMarketPath);

	const installedSourceUrls = useMemo(
		() => new Set(skills.map((s) => s.sourceUrl).filter((u): u is string => !!u)),
		[skills],
	);

	const selectedSkill: Skill | undefined = useMemo(
		() => skills.find((s) => s.id === selectedSkillId),
		[skills, selectedSkillId],
	);

	const filteredSkills = useMemo(() => {
		if (!search.trim()) return skills;
		const q = search.toLowerCase();
		return skills.filter(
			(s) =>
				s.name.toLowerCase().includes(q) ||
				(s.description?.toLowerCase().includes(q) ?? false),
		);
	}, [skills, search]);

	const filteredMarketSkills = useMemo(() => {
		if (!marketSearch.trim()) return marketSkills;
		const q = marketSearch.toLowerCase();
		return marketSkills.filter(
			(s) =>
				s.name.toLowerCase().includes(q) ||
				(s.description?.toLowerCase().includes(q) ?? false),
		);
	}, [marketSkills, marketSearch]);

	// ── Installed tab handlers ────────────────────────────────────────────────

	const handleSelectSkill = (skill: Skill) => {
		setSelectedSkillId(skill.id);
		setDeleteError(null);
	};

	const handleDelete = async () => {
		if (!selectedSkill) return;
		setIsDeleting(true);
		setDeleteError(null);
		try {
			await deleteSkill(selectedSkill.id);
			setSelectedSkillId(null);
		} catch (err) {
			if (err instanceof SkillInUseError) {
				const agentLabels =
					err.agentTypes.length > 0
						? err.agentTypes.map((t) => `"${t}"`).join(", ")
						: err.agentIds.length > 0
							? err.agentIds.join(", ")
							: "one or more agents";
				setDeleteError(
					`Cannot delete: this skill is used by ${agentLabels}. Detach it from those agents first.`,
				);
			} else if (err instanceof Error) {
				setDeleteError(err.message);
			}
		} finally {
			setIsDeleting(false);
		}
	};

	// ── Market tab handlers ───────────────────────────────────────────────────

	const handleMarketReload = async () => {
		setSelectedMarketEntry(null);
		setInstallError(null);
		setMarketSearch("");
		setIsReloading(true);
		try {
			await reloadMarket();
		} finally {
			setIsReloading(false);
		}
	};

	const closeAddRepoForm = () => {
		setShowAddRepoForm(false);
		setAddRepoForm(EMPTY_ADD_REPO_FORM);
		setAddRepoError(null);
	};

	const handleAddRepoSave = async () => {
		if (!addRepoForm.repoUrl.trim()) {
			setAddRepoError("Repo URL is required.");
			return;
		}
		setIsSavingRepo(true);
		setAddRepoError(null);
		try {
			const created = await createRepo({
				name: addRepoForm.name.trim() || addRepoForm.repoUrl.trim(),
				repoUrl: addRepoForm.repoUrl.trim(),
				branch: addRepoForm.branch.trim() || undefined,
				path: addRepoForm.path.trim() || undefined,
			});
			setActiveRepoId(created.id);
			closeAddRepoForm();
		} catch (err) {
			setAddRepoError(err instanceof Error ? err.message : "Failed to save repository.");
		} finally {
			setIsSavingRepo(false);
		}
	};

	const handleDeleteRepo = async (repoId: string) => {
		setDeletingRepoId(repoId);
		try {
			await deleteRepo(repoId);
			if (activeRepoId === repoId) setActiveRepoId(null);
		} finally {
			setDeletingRepoId(null);
		}
	};

	const handleInstallFromMarket = async (entry: SkillMarketEntry) => {
		setInstallingId(entry.id);
		setInstallError(null);
		try {
			await importSkill({
				repoUrl: effectiveMarketUrl,
				branch: effectiveMarketBranch,
				path: effectiveMarketPath,
				skillId: entry.id,
			});
			// Stay on Market tab to allow batch installs
		} catch (err) {
			setInstallError(err instanceof Error ? err.message : "Install failed");
		} finally {
			setInstallingId(null);
		}
	};

	// ── Dialog close ─────────────────────────────────────────────────────────

	const handleClose = (nextOpen: boolean) => {
		if (!nextOpen) {
			setSelectedSkillId(null);
			setSearch("");
			setDeleteError(null);
			setSelectedMarketEntry(null);
			setInstallError(null);
			setMarketSearch("");
			setActiveTab("installed");
			closeAddRepoForm();
		}
		onOpenChange(nextOpen);
	};

	// ── Render ────────────────────────────────────────────────────────────────

	return (
		<Dialog open={open} onOpenChange={handleClose}>
			<DialogContent className="sm:max-w-5xl flex flex-col min-h-[60vh] max-h-[88vh] overflow-hidden">
				<DialogHeader className="shrink-0">
					<DialogTitle>Skills</DialogTitle>
					<DialogDescription>
						Manage reusable skill packages for this project. Skills are
						directory-based and can be attached to agents.
					</DialogDescription>
				</DialogHeader>

				<div className="flex-1 min-h-0 flex flex-col mt-2">
					<Tabs
						value={activeTab}
						onValueChange={(val) => setActiveTab(val as "installed" | "market")}
						className="flex-1 min-h-0 flex flex-col"
					>
						<TabsList className="mb-3 shrink-0">
							<TabsTrigger value="installed" className="gap-1">
								<BookOpen className="h-4 w-4" /> Installed
							</TabsTrigger>
							<TabsTrigger value="market" className="gap-1">
								<Globe2 className="h-4 w-4" /> Skill Market
							</TabsTrigger>
						</TabsList>

						{/* ── Installed Tab ─────────────────────────────────── */}
						<TabsContent value="installed" className="flex-1 min-h-0 mt-0 flex flex-col">
							<div className="flex gap-4 flex-1 min-h-0 min-w-0">
								{/* Left: card grid */}
								<div className="flex-1 min-w-0 flex flex-col gap-2 min-h-0">
									{/* Search bar */}
									<div className="relative shrink-0">
										<Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
										<Input
											placeholder="Search skills..."
											value={search}
											onChange={(e) => setSearch(e.target.value)}
											className="pl-8 h-8 text-xs"
										/>
									</div>

									{/* Grid */}
									<div className="flex-1 min-h-0 overflow-y-auto">
										{isLoadingSkills ? (
											<div className="py-8 text-center text-xs text-muted-foreground">
												Loading skills...
											</div>
										) : filteredSkills.length === 0 ? (
											<div className="py-10 text-center text-xs text-muted-foreground leading-relaxed">
												{search.trim()
													? `No skills match "${search}".`
													: <>No skills installed yet.<br />Browse the Skill Market to install one.</>}
											</div>
										) : (
											<div className="grid grid-cols-2 gap-2 pb-1">
												{filteredSkills.map((skill) => (
													<button
														key={skill.id}
														type="button"
														onClick={() => handleSelectSkill(skill)}
														className={cn(
															"text-left p-3 rounded-md border bg-muted/20 hover:bg-muted/50 transition-colors",
															selectedSkillId === skill.id
																? "border-border bg-background shadow-xs"
																: "border-transparent",
														)}
													>
														<span className="font-medium text-xs block truncate">{skill.name}</span>
														{skill.description ? (
															<p className="mt-1 line-clamp-2 text-[11px] text-muted-foreground leading-relaxed">
																{skill.description}
															</p>
														) : (
															<p className="mt-1 text-[11px] text-muted-foreground/50 italic">No description</p>
														)}
													</button>
												))}
											</div>
										)}
									</div>
								</div>

								{/* Right: detail panel */}
								<div className="w-64 shrink-0 flex flex-col min-h-0 border rounded-md bg-muted/10">
									{selectedSkill ? (
										<>
											<div className="flex-1 min-h-0 overflow-y-auto p-3 space-y-2">
												<div className="space-y-0.5">
													<h3 className="font-semibold text-sm">{selectedSkill.name}</h3>
													{selectedSkill.description && (
														<p className="text-xs text-muted-foreground leading-relaxed">
															{selectedSkill.description}
														</p>
													)}
												</div>
												{selectedSkill.sourceUrl && (
													<p className="text-[10px] text-muted-foreground break-all">
														Source:{" "}
														<a
															href={selectedSkill.sourceUrl}
															target="_blank"
															rel="noopener noreferrer"
															className="underline hover:text-foreground"
														>
															{selectedSkill.sourceUrl}
														</a>
													</p>
												)}
												{deleteError && (
													<p className="text-xs text-destructive">{deleteError}</p>
												)}
											</div>
											<div className="shrink-0 px-3 py-2 border-t flex justify-end">
												<Button
													variant="outline"
													size="sm"
													disabled={isDeleting}
													onClick={handleDelete}
												>
													{isDeleting ? (
														<>
															<Loader2 className="h-3.5 w-3.5 animate-spin mr-1" />
															Deleting…
														</>
													) : (
														"Delete"
													)}
												</Button>
											</div>
										</>
									) : (
										<div className="flex-1 flex items-center justify-center text-xs text-muted-foreground text-center leading-relaxed p-3">
											Select a skill card to see details.
										</div>
									)}
								</div>
							</div>
						</TabsContent>

						{/* ── Skill Market Tab ───────────────────────────────── */}
						<TabsContent value="market" className="flex-1 min-h-0 mt-0 flex flex-col">
							<div className="flex flex-col flex-1 min-h-0 gap-3">
								{/* Repo tabs bar */}
								<div className="flex items-center gap-1.5 shrink-0 flex-wrap">
									{/* Add Repo button */}
									<button
										type="button"
										onClick={() => {
											setShowAddRepoForm((v) => !v);
											setAddRepoError(null);
										}}
										className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs border border-dashed border-muted-foreground/40 text-muted-foreground hover:bg-muted/40 focus:outline-none"
									>
										<Plus className="h-3 w-3" />
										Add Repo
									</button>

									{/* Repo tabs */}
									{!isLoadingRepos && repos.map((repo) => (
										<div
											key={repo.id}
											className={cn(
												"inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-xs border cursor-pointer select-none",
												activeRepo?.id === repo.id
													? "bg-background border-border shadow-xs font-medium"
													: "border-transparent text-muted-foreground hover:bg-muted/40",
											)}
											onClick={() => {
												setActiveRepoId(repo.id);
												setSelectedMarketEntry(null);
												setMarketSearch("");
											}}
										>
											<span className="max-w-[120px] truncate">{repo.name}</span>
											<button
												type="button"
												onClick={(e) => {
													e.stopPropagation();
													handleDeleteRepo(repo.id);
												}}
												disabled={deletingRepoId === repo.id}
												className="ml-0.5 text-muted-foreground/60 hover:text-destructive focus:outline-none"
											>
												{deletingRepoId === repo.id
													? <Loader2 className="h-3 w-3 animate-spin" />
													: <X className="h-3 w-3" />}
											</button>
										</div>
									))}

									{/* Reload button — only when a repo is active */}
									{activeRepo && (
										<button
											type="button"
											onClick={handleMarketReload}
											disabled={isReloading}
											className="ml-auto inline-flex items-center gap-1 px-2 py-1 rounded-md text-xs text-muted-foreground hover:bg-muted/40 focus:outline-none disabled:opacity-50"
										>
											{isReloading
												? <Loader2 className="h-3 w-3 animate-spin" />
												: <RefreshCw className="h-3 w-3" />}
											{isReloading ? "Loading…" : "Reload"}
										</button>
									)}
								</div>

								{/* Add-repo inline form */}
								{showAddRepoForm && (
									<div className="shrink-0 border rounded-md p-3 bg-muted/20 space-y-2">
										<p className="text-xs font-medium">Add Skill Repository</p>
										<div className="grid grid-cols-2 gap-2">
											<div className="space-y-1">
												<Label className="text-xs">Name</Label>
												<Input
													value={addRepoForm.name}
													onChange={(e) => setAddRepoForm((f) => ({ ...f, name: e.target.value }))}
													placeholder="My Skills"
													className="h-7 text-xs"
												/>
											</div>
											<div className="space-y-1">
												<Label className="text-xs">Repo URL <span className="text-destructive">*</span></Label>
												<Input
													value={addRepoForm.repoUrl}
													onChange={(e) => setAddRepoForm((f) => ({ ...f, repoUrl: e.target.value }))}
													placeholder="https://github.com/owner/repo"
													className="h-7 text-xs"
												/>
											</div>
											<div className="space-y-1">
												<Label className="text-xs">Branch</Label>
												<Input
													value={addRepoForm.branch}
													onChange={(e) => setAddRepoForm((f) => ({ ...f, branch: e.target.value }))}
													placeholder="main"
													className="h-7 text-xs"
												/>
											</div>
											<div className="space-y-1">
												<Label className="text-xs">Path</Label>
												<Input
													value={addRepoForm.path}
													onChange={(e) => setAddRepoForm((f) => ({ ...f, path: e.target.value }))}
													placeholder="skills/"
													className="h-7 text-xs"
												/>
											</div>
										</div>
										{addRepoError && (
											<p className="text-xs text-destructive">{addRepoError}</p>
										)}
										<div className="flex justify-end gap-2">
											<Button
												variant="ghost"
												size="sm"
												onClick={closeAddRepoForm}
											>
												Cancel
											</Button>
											<Button size="sm" disabled={isSavingRepo} onClick={handleAddRepoSave}>
												{isSavingRepo ? <><Loader2 className="h-3.5 w-3.5 animate-spin mr-1" />Saving…</> : "Save"}
											</Button>
										</div>
									</div>
								)}

								{/* No repos — empty state */}
								{!isLoadingRepos && repos.length === 0 && !showAddRepoForm && (
									<div className="flex-1 flex flex-col items-center justify-center gap-3 text-center text-xs text-muted-foreground">
										<Globe2 className="h-8 w-8 text-muted-foreground/30" />
										<p className="leading-relaxed">
											No skill repositories configured yet.
											<br />
											Click <strong>+ Add Repo</strong> to add your first one.
										</p>
									</div>
								)}

								{/* Two-panel layout — only when an active repo exists */}
								{activeRepo && (
									<div className="flex flex-1 min-h-0 gap-3">
										{/* Left: card grid */}
										<div className="flex-1 min-w-0 flex flex-col gap-2 min-h-0">
											{/* Search bar — shown once skills are loaded */}
											{!isLoadingMarket && marketSkills.length > 0 && (
												<div className="relative shrink-0">
													<Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
													<Input
														placeholder="Search market..."
														value={marketSearch}
														onChange={(e) => setMarketSearch(e.target.value)}
														className="pl-8 h-8 text-xs"
													/>
												</div>
											)}

											{/* Grid */}
											<div className="flex-1 min-h-0 overflow-y-auto">
												{marketError ? (
													<div className="py-10 flex flex-col items-center justify-center gap-3 text-center text-xs text-muted-foreground">
														<p className="text-destructive font-medium">Failed to load skills</p>
														<p className="text-[11px] leading-relaxed max-w-[260px]">
															{marketError instanceof Error ? marketError.message : "An unexpected error occurred."}
														</p>
														<button
															type="button"
															onClick={handleMarketReload}
															disabled={isReloading}
															className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs border hover:bg-muted/40 focus:outline-none disabled:opacity-50"
														>
															{isReloading
																? <Loader2 className="h-3 w-3 animate-spin" />
																: <RefreshCw className="h-3 w-3" />}
															{isReloading ? "Retrying…" : "Retry"}
														</button>
													</div>
												) : isLoadingMarket ? (
													<div className="py-8 text-center text-xs text-muted-foreground flex items-center justify-center gap-2">
														<Loader2 className="h-4 w-4 animate-spin" />
														Loading skills…
													</div>
												) : marketSkills.length === 0 ? (
													<div className="py-10 text-center text-xs text-muted-foreground">
														No skills found. Check the repo URL and click Reload.
													</div>
												) : filteredMarketSkills.length === 0 ? (
													<div className="py-10 text-center text-xs text-muted-foreground">
														No skills match "{marketSearch}".
													</div>
												) : (
													<div className="grid grid-cols-2 gap-2 pb-1">
														{filteredMarketSkills.map((entry) => {
															const isInstalled = installedSourceUrls.has(entry.sourceUrl);
															return (
																<button
																	key={entry.id}
																	type="button"
																	onClick={() => {
																		setSelectedMarketEntry(entry);
																		setInstallError(null);
																	}}
																	className={cn(
																		"text-left p-3 rounded-md border bg-muted/20 hover:bg-muted/50 transition-colors",
																		selectedMarketEntry?.id === entry.id
																			? "border-border bg-background shadow-xs"
																			: "border-transparent",
																	)}
																>
																	<div className="flex items-start justify-between gap-1">
																		<span className="font-medium text-xs truncate">{entry.name}</span>
																		{isInstalled ? (
																			<span className="shrink-0 text-[10px] text-green-600 dark:text-green-400 font-medium bg-green-50 dark:bg-green-950/40 px-1 py-0.5 rounded">
																				Installed
																			</span>
																		) : (
																			<span className="shrink-0 text-[10px] text-muted-foreground">
																				Available
																			</span>
																		)}
																	</div>
																	{entry.description ? (
																		<p className="mt-1 line-clamp-2 text-[11px] text-muted-foreground leading-relaxed">
																			{entry.description}
																		</p>
																	) : (
																		<p className="mt-1 text-[11px] text-muted-foreground/50 italic">No description</p>
																	)}
																</button>
															);
														})}
													</div>
												)}
											</div>
										</div>

										{/* Right: detail panel */}
										<div className="w-64 shrink-0 border rounded-md bg-muted/10 flex flex-col min-h-0">
											{selectedMarketEntry ? (
												<>
													<div className="flex-1 min-h-0 overflow-y-auto p-4 space-y-3">
														<div className="space-y-1">
															<h3 className="font-semibold text-sm">{selectedMarketEntry.name}</h3>
															{selectedMarketEntry.description && (
																<p className="text-xs text-muted-foreground leading-relaxed">
																	{selectedMarketEntry.description}
																</p>
															)}
														</div>
														<div className="space-y-1">
															{selectedMarketEntry.tags.length > 0 && (
																<div className="flex flex-wrap gap-1">
																	{selectedMarketEntry.tags.map((tag) => (
																		<span key={tag} className="text-[10px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
																			{tag}
																		</span>
																	))}
																</div>
															)}
															<p className="text-[10px] text-muted-foreground break-all">
																Source:{" "}
																<a
																	href={selectedMarketEntry.sourceUrl}
																	target="_blank"
																	rel="noopener noreferrer"
																	className="underline hover:text-foreground"
																>
																	{selectedMarketEntry.sourceUrl}
																</a>
															</p>
														</div>
														{installError && (
															<p className="text-xs text-destructive">{installError}</p>
														)}
													</div>
													<div className="shrink-0 px-4 py-3 border-t flex justify-end">
														{installedSourceUrls.has(selectedMarketEntry.sourceUrl) ? (
															<Button variant="outline" size="sm" disabled>
																Already Installed
															</Button>
														) : (
															<Button
																size="sm"
																disabled={installingId === selectedMarketEntry.id}
																onClick={() => handleInstallFromMarket(selectedMarketEntry)}
															>
																{installingId === selectedMarketEntry.id ? (
																	<>
																		<Loader2 className="h-3.5 w-3.5 animate-spin mr-1" />
																		Installing…
																	</>
																) : (
																	"Install"
																)}
															</Button>
														)}
													</div>
												</>
											) : (
												<div className="flex-1 flex items-center justify-center text-xs text-muted-foreground text-center leading-relaxed p-4">
													Select a skill card to see details.
												</div>
											)}
										</div>
									</div>
								)}
							</div>
						</TabsContent>
					</Tabs>
				</div>
			</DialogContent>
		</Dialog>
	);
}
