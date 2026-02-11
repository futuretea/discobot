import { AlertCircle } from "lucide-react";
import * as React from "react";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import type { CreateWorkspaceRequest } from "@/lib/api-types";
import { WorkspaceForm, type WorkspaceFormRef } from "../workspace-form";

interface AddWorkspaceDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onAdd: (workspace: CreateWorkspaceRequest) => Promise<void>;
}

export function AddWorkspaceDialog({
	open,
	onOpenChange,
	onAdd,
}: AddWorkspaceDialogProps) {
	const formRef = React.useRef<WorkspaceFormRef>(null);
	const [isValid, setIsValid] = React.useState(false);
	const [isSubmitting, setIsSubmitting] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);

	// Reset error when dialog opens/closes
	React.useEffect(() => {
		if (!open) {
			setError(null);
			setIsSubmitting(false);
		}
	}, [open]);

	const handleSubmit = () => {
		formRef.current?.submit();
	};

	const handleFormSubmit = async (workspace: CreateWorkspaceRequest) => {
		setIsSubmitting(true);
		setError(null);
		try {
			await onAdd(workspace);
			onOpenChange(false);
		} catch (err) {
			setError(
				err instanceof Error ? err.message : "Failed to create workspace",
			);
		} finally {
			setIsSubmitting(false);
		}
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-md">
				<DialogHeader>
					<DialogTitle>Add Workspace</DialogTitle>
					<DialogDescription>
						A workspace is a local software project. It must be a Git repository
						or cloned from a Git repository.
					</DialogDescription>
				</DialogHeader>
				<div className="py-4">
					<WorkspaceForm
						ref={formRef}
						onSubmit={handleFormSubmit}
						onValidationChange={setIsValid}
					/>
					{error && (
						<div className="mt-4 flex items-start gap-2 text-sm text-destructive bg-destructive/10 p-3 rounded-md">
							<AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
							<span>{error}</span>
						</div>
					)}
				</div>
				<DialogFooter>
					<Button
						variant="outline"
						onClick={() => onOpenChange(false)}
						disabled={isSubmitting}
					>
						Cancel
					</Button>
					<Button onClick={handleSubmit} disabled={!isValid || isSubmitting}>
						{isSubmitting ? "Creating..." : "Add Workspace"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
