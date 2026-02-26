import { FileIcon, FolderIcon, Loader2Icon } from "lucide-react";
import { memo, type RefObject, useEffect, useRef } from "react";
import type { FileMentionItem } from "@/lib/hooks/use-file-mention-search";
import { cn } from "@/lib/utils";

export interface FileMentionDropdownProps {
	isOpen: boolean;
	query: string;
	suggestions: FileMentionItem[];
	selectedIndex: number;
	isLoading: boolean;
	onSelect: (path: string) => void;
	onDismiss: () => void;
	textareaRef: RefObject<HTMLTextAreaElement | null>;
}

export const FileMentionDropdown = memo(function FileMentionDropdown({
	isOpen,
	query,
	suggestions,
	selectedIndex,
	isLoading,
	onSelect,
	onDismiss,
	textareaRef,
}: FileMentionDropdownProps) {
	const dropdownRef = useRef<HTMLDivElement>(null);

	// Scroll selected item into view
	useEffect(() => {
		if (isOpen && selectedIndex >= 0 && dropdownRef.current) {
			const selectedItem = dropdownRef.current.querySelector(
				`[data-index="${selectedIndex}"]`,
			);
			if (selectedItem && typeof selectedItem.scrollIntoView === "function") {
				selectedItem.scrollIntoView({ block: "nearest" });
			}
		}
	}, [isOpen, selectedIndex]);

	// Close on click outside
	useEffect(() => {
		if (!isOpen) return;

		const handleClickOutside = (e: MouseEvent) => {
			if (
				dropdownRef.current &&
				!dropdownRef.current.contains(e.target as Node) &&
				textareaRef.current &&
				!textareaRef.current.contains(e.target as Node)
			) {
				onDismiss();
			}
		};

		document.addEventListener("mousedown", handleClickOutside);
		return () => document.removeEventListener("mousedown", handleClickOutside);
	}, [isOpen, textareaRef, onDismiss]);

	if (!isOpen) return null;

	const showEmpty = !isLoading && suggestions.length === 0 && query.length > 0;
	const showLoading = isLoading && suggestions.length === 0;

	if (!showLoading && !showEmpty && suggestions.length === 0) return null;

	return (
		<div
			ref={dropdownRef}
			className="absolute bottom-full left-0 right-0 z-50 mb-1 flex max-h-64 flex-col overflow-hidden rounded-lg border border-border bg-popover shadow-lg"
		>
			<div className="sticky top-0 z-10 flex items-center gap-2 border-b border-border bg-popover px-3 py-2">
				<FileIcon className="h-4 w-4 text-muted-foreground" />
				<span className="text-xs font-medium text-muted-foreground">Files</span>
				<span className="ml-auto text-xs text-muted-foreground">
					↑/↓ navigate · Tab to select
				</span>
			</div>

			{showLoading && (
				<div className="flex items-center gap-2 px-3 py-3 text-xs text-muted-foreground">
					<Loader2Icon className="h-3.5 w-3.5 animate-spin" />
					Searching…
				</div>
			)}

			{showEmpty && (
				<div className="px-3 py-3 text-xs text-muted-foreground">
					No results for &ldquo;{query}&rdquo;
				</div>
			)}

			{suggestions.length > 0 && (
				<div className="overflow-y-auto py-1">
					{suggestions.map((item, index) => {
						const isDir = item.type === "directory";
						const Icon = isDir ? FolderIcon : FileIcon;
						return (
							<button
								key={item.path}
								type="button"
								data-index={index}
								className={cn(
									"flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm transition-colors",
									"hover:bg-accent",
									index === selectedIndex && "bg-accent",
								)}
								onMouseDown={(e) => {
									// Use mousedown to fire before the textarea blur
									e.preventDefault();
									onSelect(item.path);
								}}
							>
								<Icon
									className={cn(
										"h-3.5 w-3.5 shrink-0",
										isDir
											? "text-blue-400 dark:text-blue-300"
											: "text-muted-foreground",
									)}
								/>
								<span className="min-w-0 truncate font-mono text-xs">
									{item.path}
									{isDir && "/"}
								</span>
							</button>
						);
					})}
				</div>
			)}
		</div>
	);
});
