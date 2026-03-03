import {
	AlertTriangle,
	Columns2,
	Download,
	Eye,
	FileText,
	Loader2,
	Pencil,
	RotateCcw,
	Save,
	X,
} from "lucide-react";
import { useTheme } from "next-themes";
import * as React from "react";
import { lazy, Suspense } from "react";
import { useSWRConfig } from "swr";

// Lazy-load heavy Monaco editor components (~2MB)
const Editor = lazy(() =>
	import("@monaco-editor/react").then((mod) => ({ default: mod.Editor })),
);

const DiffEditor = lazy(() =>
	import("@monaco-editor/react").then((mod) => ({ default: mod.DiffEditor })),
);

const DiffEditorLoader = () => (
	<div className="flex-1 flex items-center justify-center text-muted-foreground">
		<Loader2 className="h-5 w-5 animate-spin mr-2" />
		Loading diff...
	</div>
);

import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import type { FileNode } from "@/lib/api-types";
import { useSessionViewContext } from "@/lib/contexts/session-view-context";
import { useFileEdit } from "@/lib/hooks/use-file-edit";
import {
	useSessionFileContent,
	useSessionFileDiff,
} from "@/lib/hooks/use-session-files";
import { cn } from "@/lib/utils";
import {
	countDiffLinesFast,
	DIFF_HARD_LIMIT,
	DIFF_WARNING_THRESHOLD,
	getLanguageFromPath,
	parsePatchDecorations,
	reconstructOriginalFromPatch,
} from "@/lib/utils/diff-utils";

type ViewMode = "diff" | "edit" | "markdown" | "markdown-split";

function isMarkdownFile(filePath: string): boolean {
	return /\.(md|mdx)$/i.test(filePath);
}

function isImageFile(filePath: string): boolean {
	return /\.(png|jpe?g|gif|webp|svg|ico|bmp|avif|tiff?)$/i.test(filePath);
}

function getImageMimeType(filePath: string): string {
	const ext = filePath.match(/\.([^.]+)$/)?.[1]?.toLowerCase() ?? "";
	const mimeTypes: Record<string, string> = {
		png: "image/png",
		jpg: "image/jpeg",
		jpeg: "image/jpeg",
		gif: "image/gif",
		webp: "image/webp",
		svg: "image/svg+xml",
		ico: "image/x-icon",
		bmp: "image/bmp",
		avif: "image/avif",
		tiff: "image/tiff",
		tif: "image/tiff",
	};
	return mimeTypes[ext] ?? "image/png";
}

function ImageViewer({
	content,
	encoding,
	filePath,
}: {
	content: string;
	encoding: string | undefined;
	filePath: string;
}) {
	const mimeType = getImageMimeType(filePath);
	const src =
		encoding === "base64"
			? `data:${mimeType};base64,${content}`
			: `data:${mimeType};charset=utf-8,${encodeURIComponent(content)}`;

	return (
		<div className="flex-1 flex items-center justify-center overflow-auto p-4 bg-muted/40">
			<img
				src={src}
				alt={filePath.split("/").pop() ?? filePath}
				className="max-w-full max-h-full object-contain shadow-md"
			/>
		</div>
	);
}

function MarkdownPreview({
	content,
	className,
}: {
	content: string;
	className?: string;
}) {
	return (
		<div className={cn("overflow-auto bg-background", className)}>
			<div className="p-6 max-w-3xl mx-auto text-sm text-foreground">
				<ReactMarkdown
					remarkPlugins={[remarkGfm]}
					components={{
						h1: ({ children }) => (
							<h1 className="text-2xl font-bold mt-6 mb-4 pb-2 border-b border-border text-foreground first:mt-0">
								{children}
							</h1>
						),
						h2: ({ children }) => (
							<h2 className="text-xl font-semibold mt-5 mb-3 pb-1 border-b border-border text-foreground">
								{children}
							</h2>
						),
						h3: ({ children }) => (
							<h3 className="text-lg font-semibold mt-4 mb-2 text-foreground">
								{children}
							</h3>
						),
						h4: ({ children }) => (
							<h4 className="text-base font-semibold mt-3 mb-2 text-foreground">
								{children}
							</h4>
						),
						h5: ({ children }) => (
							<h5 className="text-sm font-semibold mt-3 mb-1 text-foreground">
								{children}
							</h5>
						),
						h6: ({ children }) => (
							<h6 className="text-xs font-semibold mt-3 mb-1 text-muted-foreground">
								{children}
							</h6>
						),
						p: ({ children }) => (
							<p className="mb-4 leading-relaxed">{children}</p>
						),
						a: ({ href, children }) => (
							<a
								href={href}
								className="text-primary underline underline-offset-2 hover:opacity-80"
								target="_blank"
								rel="noopener noreferrer"
							>
								{children}
							</a>
						),
						ul: ({ children }) => (
							<ul className="mb-4 pl-6 list-disc space-y-1">{children}</ul>
						),
						ol: ({ children }) => (
							<ol className="mb-4 pl-6 list-decimal space-y-1">{children}</ol>
						),
						li: ({ children }) => (
							<li className="leading-relaxed">{children}</li>
						),
						blockquote: ({ children }) => (
							<blockquote className="border-l-4 border-border pl-4 py-1 my-4 text-muted-foreground">
								{children}
							</blockquote>
						),
						pre: ({ children }) => (
							<pre className="bg-muted rounded-md p-4 overflow-x-auto mb-4 text-xs font-mono [&>code]:!bg-transparent [&>code]:!border-0 [&>code]:!p-0 [&>code]:!rounded-none">
								{children}
							</pre>
						),
						code: ({ children }) => (
							<code className="bg-muted px-1 py-0.5 rounded text-xs font-mono border border-border/40">
								{children}
							</code>
						),
						table: ({ children }) => (
							<div className="overflow-x-auto mb-4">
								<table className="min-w-full border-collapse border border-border">
									{children}
								</table>
							</div>
						),
						thead: ({ children }) => (
							<thead className="bg-muted">{children}</thead>
						),
						tbody: ({ children }) => <tbody>{children}</tbody>,
						tr: ({ children }) => (
							<tr className="border-b border-border">{children}</tr>
						),
						th: ({ children }) => (
							<th className="px-4 py-2 text-left font-semibold border-r border-border last:border-r-0">
								{children}
							</th>
						),
						td: ({ children }) => (
							<td className="px-4 py-2 border-r border-border last:border-r-0">
								{children}
							</td>
						),
						hr: () => <hr className="border-border my-6" />,
						strong: ({ children }) => (
							<strong className="font-semibold">{children}</strong>
						),
						em: ({ children }) => <em className="italic">{children}</em>,
						img: ({ src, alt }) => (
							<img src={src} alt={alt} className="max-w-full rounded-md my-4" />
						),
					}}
				>
					{content}
				</ReactMarkdown>
			</div>
		</div>
	);
}

interface LargeDiffFallbackProps {
	lineCount: number;
	filePath: string;
	patch: string;
	onViewCurrent: () => void;
	canLoadAnyway: boolean;
	onLoadAnyway?: () => void;
}

function LargeDiffFallback({
	lineCount,
	filePath,
	patch,
	onViewCurrent,
	canLoadAnyway,
	onLoadAnyway,
}: LargeDiffFallbackProps) {
	const handleDownloadPatch = () => {
		const blob = new Blob([patch], { type: "text/plain" });
		const url = URL.createObjectURL(blob);
		const a = document.createElement("a");
		a.href = url;
		a.download = `${filePath.split("/").pop()}.patch`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		URL.revokeObjectURL(url);
	};

	return (
		<div className="flex-1 flex items-center justify-center p-8">
			<div className="max-w-md text-center space-y-4">
				<div className="flex justify-center">
					<div className="rounded-full bg-yellow-500/10 p-3">
						<AlertTriangle className="h-8 w-8 text-yellow-500" />
					</div>
				</div>
				<div>
					<h3 className="text-lg font-semibold mb-2">
						Diff Too Large to Display
					</h3>
					<p className="text-sm text-muted-foreground">
						This diff contains{" "}
						<span className="font-medium text-foreground">
							{lineCount.toLocaleString()} lines
						</span>
						, which exceeds the rendering limit. Choose an option below to view
						the changes.
					</p>
				</div>
				<div className="flex flex-col gap-2 pt-2">
					{canLoadAnyway && onLoadAnyway && (
						<Button
							variant="default"
							className="w-full justify-start"
							onClick={onLoadAnyway}
						>
							<AlertTriangle className="h-4 w-4 mr-2" />
							Load Anyway (May Be Slow)
						</Button>
					)}
					<Button
						variant="outline"
						className="w-full justify-start"
						onClick={onViewCurrent}
					>
						<FileText className="h-4 w-4 mr-2" />
						View Current File
					</Button>
					<Button
						variant="outline"
						className="w-full justify-start"
						onClick={handleDownloadPatch}
					>
						<Download className="h-4 w-4 mr-2" />
						Download as .patch File
					</Button>
				</div>
				<p className="text-xs text-muted-foreground pt-2">
					Consider using an external diff tool for very large changes.
				</p>
			</div>
		</div>
	);
}

interface DiffContentProps {
	file: FileNode;
}

export function DiffContent({ file }: DiffContentProps) {
	// View mode state (always starts in "edit" mode when file is opened/reopened)
	// Resets to "edit" when navigating away and back to this file
	const [viewMode, setViewMode] = React.useState<ViewMode>("edit");

	const { selectedSession } = useSessionViewContext();
	const { resolvedTheme } = useTheme();

	const {
		diff,
		isLoading: isDiffLoading,
		error: diffError,
	} = useSessionFileDiff(
		selectedSession?.id ?? null,
		file.id, // file.id is the file path
	);

	// Check if the file is deleted (can't edit or view current content)
	const isDeleted = file.status === "deleted" || diff?.status === "deleted";

	// Check if we should show file content instead of diff (no diff available)
	const noDiffAvailable =
		!isDiffLoading && (!diff || diff.status === "unchanged");

	// Load current file content (for edit mode, when no diff available, and for diff viewer)
	// Don't load content for deleted files
	const shouldLoadContent =
		!isDeleted && (viewMode === "edit" || noDiffAvailable || !!diff);
	const {
		content: currentContent,
		encoding: currentEncoding,
		isLoading: isContentLoading,
		error: contentError,
	} = useSessionFileContent(
		shouldLoadContent ? (selectedSession?.id ?? null) : null,
		shouldLoadContent ? file.id : null,
	);

	// Don't wait for original content - show diff immediately, expansion is optional
	const isLoading =
		isDiffLoading ||
		(noDiffAvailable && isContentLoading) ||
		(viewMode === "edit" && isContentLoading);

	// Fast count of diff lines to determine if it's too large to render
	// This uses a simple string scan without parsing, so it's fast even for huge diffs
	const diffLineCount = React.useMemo(() => {
		if (!diff?.patch) return 0;
		return countDiffLinesFast(diff.patch);
	}, [diff?.patch]);

	// Track whether user wants to force load a large diff
	const [forceLoadLargeDiff, setForceLoadLargeDiff] = React.useState(false);

	// Track previous file ID to detect changes
	const prevFileIdRef = React.useRef(file.id);

	// Reset force load when file changes
	React.useEffect(() => {
		if (prevFileIdRef.current !== file.id) {
			setForceLoadLargeDiff(false);
			prevFileIdRef.current = file.id;
		}
	});

	// Check if diff is too large BEFORE expensive operations
	const isOverHardLimit = diffLineCount > DIFF_HARD_LIMIT;
	const isOverWarningThreshold =
		diffLineCount > DIFF_WARNING_THRESHOLD && diffLineCount <= DIFF_HARD_LIMIT;
	const shouldShowFallback =
		(isOverWarningThreshold && !forceLoadLargeDiff) || isOverHardLimit;

	// Only reconstruct original content if we're actually going to render Monaco
	// This is expensive for large diffs, so skip it when showing fallback
	const originalContent = React.useMemo(() => {
		// Skip expensive computation if we're showing fallback
		if (shouldShowFallback) {
			return "";
		}

		if (file.status === "added") {
			// For new files, original is empty
			return "";
		}
		if (isDeleted) {
			// For deleted files, reconstruct original from patch.
			// The patch describes original → empty, so reversing it with "" recovers the original.
			if (!diff?.patch) return "";
			return reconstructOriginalFromPatch("", diff.patch);
		}
		if (!currentContent || !diff?.patch) {
			return "";
		}
		return reconstructOriginalFromPatch(currentContent, diff.patch);
	}, [currentContent, diff?.patch, file.status, isDeleted, shouldShowFallback]);

	const language = getLanguageFromPath(file.id);

	// Integrate useFileEdit hook for editing functionality in diff mode
	const fileEdit = useFileEdit(
		selectedSession?.id ?? null,
		file.id,
		currentContent,
		isContentLoading,
	);

	// Use ref to access latest fileEdit in onMount callback
	const fileEditRef = React.useRef(fileEdit);

	React.useEffect(() => {
		fileEditRef.current = fileEdit;
	}, [fileEdit]);

	// Track conflict dialog state for diff mode
	const [showConflictDialog, setShowConflictDialog] = React.useState(false);

	// Get SWR mutate for refreshing diff data after save
	const { mutate } = useSWRConfig();

	// Keyboard shortcut: Cmd+S / Ctrl+S to save in diff mode
	React.useEffect(() => {
		if (viewMode !== "diff") return;

		const handleKeyDown = (e: KeyboardEvent) => {
			if ((e.metaKey || e.ctrlKey) && e.key === "s") {
				e.preventDefault();
				if (fileEdit.state.isDirty && !fileEdit.state.isSaving) {
					fileEdit.save().then((success) => {
						if (success && selectedSession?.id) {
							// Refresh diff data after save
							setTimeout(() => {
								mutate(`session-diff-${selectedSession.id}-files`);
								mutate(`session-diff-${selectedSession.id}-${file.id}`);
							}, 100);
						}
					});
				}
			}
		};

		window.addEventListener("keydown", handleKeyDown);
		return () => window.removeEventListener("keydown", handleKeyDown);
	}, [viewMode, fileEdit, selectedSession?.id, file.id, mutate]);

	if (isLoading) {
		return (
			<div className="flex-1 flex items-center justify-center text-muted-foreground">
				<Loader2 className="h-5 w-5 animate-spin mr-2" />
				{isDiffLoading ? "Loading diff..." : "Loading file..."}
			</div>
		);
	}

	if (diffError && !noDiffAvailable) {
		return (
			<div className="flex-1 flex items-center justify-center text-destructive">
				Failed to load diff: {diffError.message}
			</div>
		);
	}

	// Image files: render inline viewer instead of Monaco editor
	if (isImageFile(file.id) && !isDeleted) {
		if (contentError) {
			return (
				<div className="flex-1 flex items-center justify-center text-destructive">
					Failed to load image: {contentError.message}
				</div>
			);
		}
		if (currentContent !== undefined && currentContent !== null) {
			return (
				<ImageViewer
					content={currentContent}
					encoding={currentEncoding}
					filePath={file.id}
				/>
			);
		}
	}

	// Show file content when no diff available or in edit/markdown modes
	if (
		noDiffAvailable ||
		viewMode === "edit" ||
		viewMode === "markdown" ||
		viewMode === "markdown-split"
	) {
		if (contentError) {
			return (
				<div className="flex-1 flex items-center justify-center text-destructive">
					Failed to load file: {contentError.message}
				</div>
			);
		}

		if (currentContent === undefined || currentContent === null) {
			return (
				<div className="flex-1 flex items-center justify-center text-muted-foreground">
					No content available
				</div>
			);
		}

		// When noDiffAvailable and viewMode is still "diff" (default), treat as "edit"
		const fileViewMode =
			noDiffAvailable && viewMode === "diff" ? "edit" : viewMode;

		return (
			<FileContentView
				content={currentContent}
				filePath={file.id}
				isServerLoading={isContentLoading}
				onBackToDiff={!noDiffAvailable ? () => setViewMode("diff") : undefined}
				viewMode={fileViewMode}
				onViewModeChange={setViewMode}
				patch={diff?.patch}
			/>
		);
	}

	if (!diff || !diff.patch) {
		return (
			<div className="flex-1 flex items-center justify-center text-muted-foreground">
				No diff available
			</div>
		);
	}

	if (diff.binary) {
		return (
			<div className="flex-1 flex items-center justify-center text-muted-foreground">
				Binary file - cannot display diff
			</div>
		);
	}

	// Show fallback if diff is too large (computed earlier to avoid expensive operations)
	if (shouldShowFallback) {
		return (
			<LargeDiffFallback
				lineCount={diffLineCount}
				filePath={file.id}
				patch={diff.patch}
				onViewCurrent={() => setViewMode("edit")}
				canLoadAnyway={isOverWarningThreshold && !isOverHardLimit}
				onLoadAnyway={
					isOverWarningThreshold
						? () => {
								// Use startTransition to prevent blocking the UI
								// while React prepares the expensive Monaco render
								React.startTransition(() => {
									setForceLoadLargeDiff(true);
								});
							}
						: undefined
				}
			/>
		);
	}

	return (
		<div className="flex-1 flex flex-col overflow-hidden">
			{/* Diff header toolbar */}
			<div className="h-8 flex items-center justify-between px-2 border-b border-border bg-muted/20 shrink-0">
				<div className="flex items-center gap-3">
					{/* Show large diff warning if force loaded */}
					{forceLoadLargeDiff && (
						<span className="text-xs text-yellow-500 font-medium flex items-center gap-1">
							<AlertTriangle className="h-3 w-3" />
							Large diff ({diffLineCount.toLocaleString()} lines)
						</span>
					)}
					{/* Show status badge */}
					{file.status === "added" && (
						<span className="text-xs text-green-500 font-medium">New File</span>
					)}
					{isDeleted && (
						<span className="text-xs text-red-500 font-medium">
							File Deleted
						</span>
					)}
					{file.status === "renamed" && (
						<span className="text-xs text-purple-500 font-medium">Renamed</span>
					)}
					{/* Show conflict indicator */}
					{fileEdit.state.hasConflict && (
						<button
							type="button"
							className="flex items-center gap-1 text-xs text-yellow-500 hover:underline"
							onClick={() => setShowConflictDialog(true)}
							title="Click to resolve conflict"
						>
							<AlertTriangle className="h-3 w-3" />
							Conflict
						</button>
					)}
					{/* Show modified indicator when dirty */}
					{fileEdit.state.isDirty && (
						<span className="text-xs text-muted-foreground">Modified</span>
					)}
				</div>
				<div className="flex items-center gap-1">
					{/* Save/Discard buttons - only show when dirty and for non-deleted files */}
					{!isDeleted && fileEdit.state.isDirty && (
						<>
							<Button
								variant="ghost"
								size="sm"
								className="h-6 px-2 text-xs"
								onClick={() => fileEdit.discard()}
								disabled={fileEdit.state.isSaving}
								title="Discard changes"
							>
								<X className="h-3 w-3 mr-1" />
								Discard
							</Button>
							<Button
								variant="default"
								size="sm"
								className="h-6 px-2 text-xs"
								onClick={async () => {
									const success = await fileEdit.save();
									if (success && selectedSession?.id) {
										// Refresh diff data after save
										setTimeout(() => {
											mutate(`session-diff-${selectedSession.id}-files`);
											mutate(`session-diff-${selectedSession.id}-${file.id}`);
										}, 100);
									}
								}}
								disabled={fileEdit.state.isSaving}
								title="Save changes (Cmd/Ctrl+S)"
							>
								{fileEdit.state.isSaving ? (
									<Loader2 className="h-3 w-3 mr-1 animate-spin" />
								) : (
									<Save className="h-3 w-3 mr-1" />
								)}
								Save
							</Button>
						</>
					)}
					{/* Markdown preview buttons for markdown files (when not dirty) */}
					{!isDeleted && !fileEdit.state.isDirty && isMarkdownFile(file.id) && (
						<>
							<Button
								variant="ghost"
								size="sm"
								className="h-6 px-2 text-xs"
								onClick={() => setViewMode("markdown-split")}
								title="Split view: editor + preview"
							>
								<Columns2 className="h-3 w-3 mr-1" />
								Split
							</Button>
							<Button
								variant="ghost"
								size="sm"
								className="h-6 px-2 text-xs"
								onClick={() => setViewMode("markdown")}
								title="Markdown preview"
							>
								<Eye className="h-3 w-3 mr-1" />
								Preview
							</Button>
						</>
					)}
					{/* Show Editor button to switch to edit mode for non-deleted files (when not dirty) */}
					{!isDeleted && !fileEdit.state.isDirty && (
						<Button
							variant="ghost"
							size="sm"
							className="h-6 px-2 text-xs"
							onClick={() => setViewMode("edit")}
							title="Switch to editor"
						>
							<Pencil className="h-3 w-3 mr-1" />
							Editor
						</Button>
					)}
				</div>
			</div>
			{/* Monaco DiffEditor */}
			<div className="flex-1 overflow-hidden">
				<Suspense fallback={<DiffEditorLoader />}>
					<DiffEditor
						key={`diff-${file.id}`}
						height="100%"
						language={language}
						original={originalContent}
						modified={isDeleted ? "" : fileEdit.state.content || ""}
						theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
						beforeMount={(monaco) => {
							// Disable TypeScript/JavaScript validation
							monaco.languages.typescript.typescriptDefaults.setDiagnosticsOptions(
								{
									noSemanticValidation: true,
									noSyntaxValidation: false, // Keep syntax highlighting
								},
							);
							monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions(
								{
									noSemanticValidation: true,
									noSyntaxValidation: false, // Keep syntax highlighting
								},
							);
						}}
						options={{
							readOnly: isDeleted,
							renderSideBySide: true,
							minimap: { enabled: false },
							scrollBeyondLastLine: false,
							fontSize: 13,
							lineNumbers: "on",
							renderLineHighlight: "all",
							scrollbar: {
								verticalScrollbarSize: 10,
								horizontalScrollbarSize: 10,
							},
							// Collapse unchanged regions for easier navigation
							hideUnchangedRegions: {
								enabled: true,
								minimumLineCount: 3,
								contextLineCount: 3,
								revealLineCount: 0, // Start with regions collapsed
							},
							diffWordWrap: "on",
						}}
						onMount={(editor) => {
							// Get the modified (right-side) editor
							const modifiedEditor = editor.getModifiedEditor();

							// Attach change listener
							const disposable = modifiedEditor.onDidChangeModelContent(() => {
								// Use ref to get latest fileEdit
								const currentFileEdit = fileEditRef.current;

								// Always handle edits (unless it's a deleted file)
								if (!isDeleted) {
									const value = modifiedEditor.getValue();
									currentFileEdit.handleEdit(value);
								}
							});

							// Clean up on unmount
							return () => disposable.dispose();
						}}
					/>
				</Suspense>
			</div>

			{/* Conflict Resolution Dialog for Diff Mode */}
			<Dialog open={showConflictDialog} onOpenChange={setShowConflictDialog}>
				<DialogContent className="max-w-4xl h-[80vh] flex flex-col">
					<DialogHeader>
						<DialogTitle className="flex items-center gap-2">
							<AlertTriangle className="h-5 w-5 text-yellow-500" />
							File Modified Externally
						</DialogTitle>
						<DialogDescription>
							This file was modified while you were editing. Review the changes
							below and choose how to resolve the conflict.
						</DialogDescription>
					</DialogHeader>

					{/* Diff view showing server (left) vs local (right) */}
					<div className="flex-1 min-h-0 overflow-hidden border rounded-md">
						{fileEdit.state.conflictContent !== null && (
							<Suspense fallback={<DiffEditorLoader />}>
								<DiffEditor
									key={`conflict-diff-${file.id}`}
									height="100%"
									language={language}
									original={fileEdit.state.conflictContent}
									modified={fileEdit.state.content}
									theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
									beforeMount={(monaco) => {
										// Disable TypeScript/JavaScript validation
										monaco.languages.typescript.typescriptDefaults.setDiagnosticsOptions(
											{
												noSemanticValidation: true,
												noSyntaxValidation: false, // Keep syntax highlighting
											},
										);
										monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions(
											{
												noSemanticValidation: true,
												noSyntaxValidation: false, // Keep syntax highlighting
											},
										);
									}}
									options={{
										readOnly: true,
										renderSideBySide: true,
										minimap: { enabled: false },
										scrollBeyondLastLine: false,
										fontSize: 13,
										lineNumbers: "on",
										renderLineHighlight: "all",
										scrollbar: {
											verticalScrollbarSize: 10,
											horizontalScrollbarSize: 10,
										},
										// Collapse unchanged regions for easier navigation
										hideUnchangedRegions: {
											enabled: true,
											minimumLineCount: 3,
											contextLineCount: 3,
											revealLineCount: 0, // Start with regions collapsed
										},
										diffWordWrap: "on",
									}}
								/>
							</Suspense>
						)}
					</div>

					<DialogFooter className="flex-row justify-between sm:justify-between gap-2">
						<Button
							variant="outline"
							onClick={() => setShowConflictDialog(false)}
						>
							Keep Editing
						</Button>
						<div className="flex gap-2">
							<Button
								variant="secondary"
								onClick={() => {
									fileEdit.acceptServerContent();
									setShowConflictDialog(false);
								}}
							>
								Use Disk Version
							</Button>
							<Button
								onClick={async () => {
									const success = await fileEdit.forceSave();
									if (success && selectedSession?.id) {
										mutate(`session-diff-${selectedSession.id}-files`);
										mutate(`session-diff-${selectedSession.id}-${file.id}`);
										setShowConflictDialog(false);
									}
								}}
							>
								<Save className="h-4 w-4 mr-2" />
								Save My Changes
							</Button>
						</div>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}

function FileContentView({
	content: serverContent,
	filePath,
	isServerLoading,
	onBackToDiff,
	viewMode,
	onViewModeChange,
	patch,
}: {
	content: string;
	filePath: string;
	isServerLoading?: boolean;
	/** Callback to return to diff view (undefined if no diff available) */
	onBackToDiff?: () => void;
	viewMode: ViewMode;
	onViewModeChange: (mode: ViewMode) => void;
	/** Unified diff patch to highlight modified lines in the editor */
	patch?: string;
}) {
	const isMarkdown = isMarkdownFile(filePath);
	const { selectedSession } = useSessionViewContext();
	const { resolvedTheme } = useTheme();
	const { mutate } = useSWRConfig();
	const language = getLanguageFromPath(filePath);

	const { state, handleEdit, save, acceptServerContent, forceSave, discard } =
		useFileEdit(
			selectedSession?.id ?? null,
			filePath,
			serverContent,
			isServerLoading ?? false,
		);

	// Parse patch into line ranges for Monaco gutter decorations
	const patchRanges = React.useMemo(
		() => (patch ? parsePatchDecorations(patch) : null),
		[patch],
	);

	// Track if conflict dialog has been dismissed (to allow continued editing)
	const [conflictDismissed, setConflictDismissed] = React.useState(false);

	// Reset dismissed state when conflict is resolved
	React.useEffect(() => {
		if (!state.hasConflict) {
			setConflictDismissed(false);
		}
	}, [state.hasConflict]);

	const handleEditorChange = React.useCallback(
		(value: string | undefined) => {
			if (value !== undefined) {
				handleEdit(value);
			}
		},
		[handleEdit],
	);

	const handleSave = React.useCallback(async () => {
		const success = await save();
		if (success && selectedSession?.id) {
			// Refresh diff data after successful save
			mutate(`session-diff-${selectedSession.id}-files`);
		}
	}, [save, selectedSession?.id, mutate]);

	const handleForceSave = React.useCallback(async () => {
		const success = await forceSave();
		if (success && selectedSession?.id) {
			// Refresh diff data after successful save
			mutate(`session-diff-${selectedSession.id}-files`);
		}
	}, [forceSave, selectedSession?.id, mutate]);

	const handleDiscard = React.useCallback(() => {
		discard();
	}, [discard]);

	// Keyboard shortcut for save
	React.useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if ((e.metaKey || e.ctrlKey) && e.key === "s") {
				e.preventDefault();
				if (state.isDirty && !state.isSaving) {
					handleSave();
				}
			}
		};
		window.addEventListener("keydown", handleKeyDown);
		return () => window.removeEventListener("keydown", handleKeyDown);
	}, [state.isDirty, state.isSaving, handleSave]);

	return (
		<div className="flex-1 flex flex-col overflow-hidden">
			{/* Editor toolbar */}
			<div className="h-8 flex items-center justify-between px-2 border-b border-border bg-muted/20 shrink-0">
				<div className="flex items-center gap-3">
					{state.isDirty && (
						<span className="text-xs text-muted-foreground">Modified</span>
					)}
					{state.saveError && !state.hasConflict && (
						<span className="text-xs text-destructive">{state.saveError}</span>
					)}
				</div>
				<div className="flex items-center gap-1">
					{/* Markdown mode toggle buttons for markdown files */}
					{isMarkdown && (
						<>
							<Button
								variant={viewMode === "edit" ? "secondary" : "ghost"}
								size="sm"
								className="h-6 px-2 text-xs"
								onClick={() => onViewModeChange("edit")}
								title="Edit mode"
							>
								<Pencil className="h-3 w-3 mr-1" />
								Edit
							</Button>
							<Button
								variant={viewMode === "markdown-split" ? "secondary" : "ghost"}
								size="sm"
								className="h-6 px-2 text-xs"
								onClick={() => onViewModeChange("markdown-split")}
								title="Split view: editor + preview"
							>
								<Columns2 className="h-3 w-3 mr-1" />
								Split
							</Button>
							<Button
								variant={viewMode === "markdown" ? "secondary" : "ghost"}
								size="sm"
								className="h-6 px-2 text-xs"
								onClick={() => onViewModeChange("markdown")}
								title="Markdown preview"
							>
								<Eye className="h-3 w-3 mr-1" />
								Preview
							</Button>
						</>
					)}
					{/* Switch to diff view button */}
					{onBackToDiff && !state.isDirty && (
						<Button
							variant="ghost"
							size="sm"
							className="h-6 px-2 text-xs"
							onClick={onBackToDiff}
							title="Switch to diff view"
						>
							Diff View
						</Button>
					)}
					{state.isDirty && (
						<>
							<Button
								variant="ghost"
								size="sm"
								className="h-6 px-2 text-xs"
								onClick={handleDiscard}
								disabled={state.isSaving}
								title="Discard changes"
							>
								<RotateCcw className="h-3 w-3 mr-1" />
								Discard
							</Button>
							<Button
								variant="ghost"
								size="sm"
								className="h-6 px-2 text-xs"
								onClick={handleSave}
								disabled={state.isSaving}
								title="Save (Cmd+S)"
							>
								{state.isSaving ? (
									<Loader2 className="h-3 w-3 mr-1 animate-spin" />
								) : (
									<Save className="h-3 w-3 mr-1" />
								)}
								Save
							</Button>
						</>
					)}
				</div>
			</div>

			{/* Content: editor, split, or markdown preview */}
			{viewMode === "markdown" ? (
				<MarkdownPreview content={state.content} className="flex-1" />
			) : viewMode === "markdown-split" ? (
				<div className="flex-1 flex overflow-hidden">
					<div className="flex-1 min-w-0 overflow-hidden border-r border-border">
						<Editor
							key={`edit-${filePath}`}
							height="100%"
							language={language}
							value={state.content}
							onChange={handleEditorChange}
							theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
							beforeMount={(monaco) => {
								// Disable TypeScript/JavaScript validation
								monaco.languages.typescript.typescriptDefaults.setDiagnosticsOptions(
									{
										noSemanticValidation: true,
										noSyntaxValidation: false, // Keep syntax highlighting
									},
								);
								monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions(
									{
										noSemanticValidation: true,
										noSyntaxValidation: false, // Keep syntax highlighting
									},
								);
							}}
							onMount={(editor, monacoInstance) => {
								if (patchRanges && patchRanges.length > 0) {
									editor.createDecorationsCollection(
										patchRanges.map(({ startLine, endLine, type }) => ({
											range: new monacoInstance.Range(startLine, 1, endLine, 1),
											options: {
												isWholeLine: true,
												className:
													type === "added"
														? "monaco-diff-decoration-added"
														: "monaco-diff-decoration-modified",
												linesDecorationsClassName:
													type === "added"
														? "monaco-diff-gutter-added"
														: "monaco-diff-gutter-modified",
											},
										})),
									);
								}
							}}
							options={{
								readOnly: false,
								minimap: { enabled: false },
								scrollBeyondLastLine: false,
								fontSize: 13,
								lineNumbers: "on",
								renderLineHighlight: "line",
								scrollbar: {
									verticalScrollbarSize: 10,
									horizontalScrollbarSize: 10,
								},
								padding: { top: 8 },
							}}
							loading={
								<div className="flex-1 flex items-center justify-center text-muted-foreground">
									<Loader2 className="h-5 w-5 animate-spin mr-2" />
									Loading editor...
								</div>
							}
						/>
					</div>
					<MarkdownPreview content={state.content} className="flex-1 min-w-0" />
				</div>
			) : (
				<div className="flex-1 overflow-hidden">
					<Editor
						key={`edit-${filePath}`}
						height="100%"
						language={language}
						value={state.content}
						onChange={handleEditorChange}
						theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
						beforeMount={(monaco) => {
							// Disable TypeScript/JavaScript validation
							monaco.languages.typescript.typescriptDefaults.setDiagnosticsOptions(
								{
									noSemanticValidation: true,
									noSyntaxValidation: false, // Keep syntax highlighting
								},
							);
							monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions(
								{
									noSemanticValidation: true,
									noSyntaxValidation: false, // Keep syntax highlighting
								},
							);
						}}
						onMount={(editor, monacoInstance) => {
							if (patchRanges && patchRanges.length > 0) {
								editor.createDecorationsCollection(
									patchRanges.map(({ startLine, endLine, type }) => ({
										range: new monacoInstance.Range(startLine, 1, endLine, 1),
										options: {
											isWholeLine: true,
											className:
												type === "added"
													? "monaco-diff-decoration-added"
													: "monaco-diff-decoration-modified",
											linesDecorationsClassName:
												type === "added"
													? "monaco-diff-gutter-added"
													: "monaco-diff-gutter-modified",
										},
									})),
								);
							}
						}}
						options={{
							readOnly: false,
							minimap: { enabled: false },
							scrollBeyondLastLine: false,
							fontSize: 13,
							lineNumbers: "on",
							renderLineHighlight: "line",
							scrollbar: {
								verticalScrollbarSize: 10,
								horizontalScrollbarSize: 10,
							},
							padding: { top: 8 },
						}}
						loading={
							<div className="flex-1 flex items-center justify-center text-muted-foreground">
								<Loader2 className="h-5 w-5 animate-spin mr-2" />
								Loading editor...
							</div>
						}
					/>
				</div>
			)}

			{/* Conflict Resolution Dialog */}
			<Dialog
				open={state.hasConflict && !conflictDismissed}
				onOpenChange={(open) => !open && setConflictDismissed(true)}
			>
				<DialogContent className="max-w-4xl max-h-[80vh] flex flex-col">
					<DialogHeader>
						<DialogTitle className="flex items-center gap-2">
							<AlertTriangle className="h-5 w-5 text-yellow-500" />
							File Modified Externally
						</DialogTitle>
						<DialogDescription>
							This file was modified while you were editing. Review the changes
							below and choose how to resolve the conflict.
						</DialogDescription>
					</DialogHeader>

					{/* Diff view showing server (left) vs local (right) */}
					<div className="flex-1 min-h-0 overflow-hidden border rounded-md">
						{state.conflictContent !== null && (
							<DiffEditor
								key={`conflict-diff-${filePath}`}
								height="100%"
								language={language}
								original={state.conflictContent}
								modified={state.content}
								theme={resolvedTheme === "dark" ? "vs-dark" : "vs"}
								beforeMount={(monaco) => {
									// Disable TypeScript/JavaScript validation
									monaco.languages.typescript.typescriptDefaults.setDiagnosticsOptions(
										{
											noSemanticValidation: true,
											noSyntaxValidation: false, // Keep syntax highlighting
										},
									);
									monaco.languages.typescript.javascriptDefaults.setDiagnosticsOptions(
										{
											noSemanticValidation: true,
											noSyntaxValidation: false, // Keep syntax highlighting
										},
									);
								}}
								options={{
									readOnly: true,
									renderSideBySide: true,
									minimap: { enabled: false },
									scrollBeyondLastLine: false,
									fontSize: 13,
									lineNumbers: "on",
									renderLineHighlight: "all",
									scrollbar: {
										verticalScrollbarSize: 10,
										horizontalScrollbarSize: 10,
									},
									// Collapse unchanged regions for easier navigation
									hideUnchangedRegions: {
										enabled: true,
										minimumLineCount: 3,
										contextLineCount: 3,
										revealLineCount: 0, // Start with regions collapsed
									},
									diffWordWrap: "on",
								}}
							/>
						)}
					</div>

					<DialogFooter className="flex-row justify-between sm:justify-between gap-2">
						<Button
							variant="outline"
							onClick={() => setConflictDismissed(true)}
						>
							Keep Editing
						</Button>
						<div className="flex gap-2">
							<Button variant="secondary" onClick={acceptServerContent}>
								Use Disk Version
							</Button>
							<Button onClick={handleForceSave}>
								<Save className="h-4 w-4 mr-2" />
								Save My Changes
							</Button>
						</div>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}
