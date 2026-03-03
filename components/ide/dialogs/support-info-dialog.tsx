import { Copy, Download, Info } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { api } from "@/lib/api-client";
import { isTauri } from "@/lib/api-config";
import type { SupportInfoResponse } from "@/lib/api-types";

interface SupportInfoDialogProps {
	open: boolean;
	onClose: () => void;
}

/**
 * Dialog that displays diagnostic information for debugging and support.
 * Shows version, runtime info, config, and server logs.
 */
export function SupportInfoDialog({ open, onClose }: SupportInfoDialogProps) {
	const [supportInfo, setSupportInfo] =
		React.useState<SupportInfoResponse | null>(null);
	const [isLoading, setIsLoading] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);

	// Fetch support info when dialog opens
	React.useEffect(() => {
		if (open) {
			setIsLoading(true);
			setError(null);
			api
				.getSupportInfo()
				.then((data) => {
					setSupportInfo(data);
					setIsLoading(false);
				})
				.catch((err) => {
					setError(err.message);
					setIsLoading(false);
				});
		}
	}, [open]);

	const handleCopy = () => {
		if (!supportInfo) return;
		const text = JSON.stringify(supportInfo, null, 2);
		navigator.clipboard.writeText(text);
	};

	const handleDownload = async () => {
		if (!supportInfo) return;
		const text = JSON.stringify(supportInfo, null, 2);
		const filename = `discobot-support-info-${new Date().toISOString().split("T")[0]}.json`;
		if (isTauri()) {
			const { invoke } = await import("@tauri-apps/api/core");
			await invoke("save_file_to_downloads", {
				filename,
				content: Array.from(new TextEncoder().encode(text)),
			});
			toast.success(`${filename} saved to Downloads`);
		} else {
			const blob = new Blob([text], { type: "application/json" });
			const url = URL.createObjectURL(blob);
			const a = document.createElement("a");
			a.href = url;
			a.download = filename;
			document.body.appendChild(a);
			a.click();
			document.body.removeChild(a);
			URL.revokeObjectURL(url);
		}
	};

	return (
		<Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
			<DialogContent className="sm:max-w-3xl max-h-[90vh] flex flex-col">
				<DialogHeader>
					<DialogTitle className="flex items-center gap-2">
						<Info className="h-5 w-5" />
						Support Information
					</DialogTitle>
					<DialogDescription>
						Diagnostic information for debugging and support
					</DialogDescription>
				</DialogHeader>

				<div className="flex-1 min-h-0 -mx-6 px-6 overflow-y-auto">
					{isLoading && (
						<div className="py-8 text-center text-muted-foreground">
							Loading...
						</div>
					)}

					{error && (
						<div className="py-4 px-4 rounded-lg bg-destructive/10 border border-destructive/20 text-destructive">
							Error: {error}
						</div>
					)}

					{supportInfo && (
						<div className="space-y-4 pb-4">
							{/* Version */}
							<Section title="Version">
								<KeyValue label="Version" value={supportInfo.version} />
							</Section>

							{/* Runtime */}
							<Section title="Runtime">
								<KeyValue label="OS" value={supportInfo.runtime.os} />
								<KeyValue label="Arch" value={supportInfo.runtime.arch} />
								<KeyValue
									label="Go Version"
									value={supportInfo.runtime.go_version}
								/>
								<KeyValue
									label="CPU Count"
									value={supportInfo.runtime.num_cpu.toString()}
								/>
								<KeyValue
									label="Goroutines"
									value={supportInfo.runtime.num_goroutine.toString()}
								/>
							</Section>

							{/* Config */}
							<Section title="Configuration">
								<KeyValue
									label="Port"
									value={supportInfo.config.port.toString()}
								/>
								<KeyValue
									label="Database"
									value={supportInfo.config.database_driver}
								/>
								<KeyValue
									label="Auth Enabled"
									value={supportInfo.config.auth_enabled ? "Yes" : "No"}
								/>
								<KeyValue
									label="Tauri Mode"
									value={supportInfo.config.tauri_mode ? "Yes" : "No"}
								/>
								<KeyValue
									label="SSH Enabled"
									value={supportInfo.config.ssh_enabled ? "Yes" : "No"}
								/>
								{supportInfo.config.ssh_enabled && (
									<KeyValue
										label="SSH Port"
										value={supportInfo.config.ssh_port.toString()}
									/>
								)}
								<KeyValue
									label="Dispatcher"
									value={
										supportInfo.config.dispatcher_enabled
											? "Enabled"
											: "Disabled"
									}
								/>
								<KeyValue
									label="Sandbox Image"
									value={supportInfo.config.sandbox_image}
								/>
								<KeyValue
									label="Available Providers"
									value={
										supportInfo.config.available_providers?.join(", ") || "None"
									}
								/>
							</Section>

							{/* VZ Info */}
							{supportInfo.config.vz && (
								<Section title="VZ Provider (macOS Virtualization)">
									<KeyValue
										label="Image"
										value={supportInfo.config.vz.image_ref}
									/>
									<KeyValue
										label="Data Directory"
										value={supportInfo.config.vz.data_dir}
									/>
									<KeyValue
										label="CPU Count"
										value={
											supportInfo.config.vz.cpu_count === 0
												? "Auto (all)"
												: supportInfo.config.vz.cpu_count.toString()
										}
									/>
									<KeyValue
										label="Memory"
										value={
											supportInfo.config.vz.memory_mb === 0
												? "Auto (half)"
												: `${supportInfo.config.vz.memory_mb} MB`
										}
									/>
									<KeyValue
										label="Data Disk Size"
										value={
											supportInfo.config.vz.data_disk_gb === 0
												? "100 GB (default)"
												: `${supportInfo.config.vz.data_disk_gb} GB`
										}
									/>
									{supportInfo.config.vz.kernel_path && (
										<KeyValue
											label="Kernel Path"
											value={supportInfo.config.vz.kernel_path}
										/>
									)}
									{supportInfo.config.vz.initrd_path && (
										<KeyValue
											label="Initrd Path"
											value={supportInfo.config.vz.initrd_path}
										/>
									)}
									{supportInfo.config.vz.base_disk_path && (
										<KeyValue
											label="Base Disk Path"
											value={supportInfo.config.vz.base_disk_path}
										/>
									)}
									{supportInfo.config.vz.disk_usage && (
										<>
											<div className="pt-2 font-medium text-sm">Disk Usage</div>
											<KeyValue
												label="Total Space"
												value={formatBytes(
													supportInfo.config.vz.disk_usage.total_bytes,
												)}
											/>
											<KeyValue
												label="Used Space"
												value={`${formatBytes(supportInfo.config.vz.disk_usage.used_bytes)} (${supportInfo.config.vz.disk_usage.used_percent.toFixed(1)}%)`}
											/>
											<KeyValue
												label="Available Space"
												value={formatBytes(
													supportInfo.config.vz.disk_usage.available_bytes,
												)}
											/>
										</>
									)}
									{supportInfo.config.vz.data_disks &&
										supportInfo.config.vz.data_disks.length > 0 && (
											<>
												<div className="pt-2 font-medium text-sm">
													Data Disks
												</div>
												{supportInfo.config.vz.data_disks.map((disk) => (
													<div key={disk.path} className="space-y-1.5">
														<KeyValue
															label="File"
															value={disk.path.split("/").pop() || disk.path}
														/>
														<KeyValue
															label="File Size"
															value={formatBytes(disk.apparent_bytes)}
														/>
														<KeyValue
															label="Actual Disk Usage"
															value={formatBytes(disk.actual_bytes)}
														/>
													</div>
												))}
											</>
										)}
								</Section>
							)}

							{/* System Info */}
							{supportInfo.system_info.messages &&
								supportInfo.system_info.messages.length > 0 && (
									<Section title="System Status">
										<div className="space-y-2">
											{supportInfo.system_info.messages.map((msg) => (
												<div
													key={msg.id}
													className="p-2 rounded bg-yellow-500/10 border border-yellow-500/20"
												>
													<p className="font-medium text-sm text-yellow-600 dark:text-yellow-400">
														{msg.title}
													</p>
													<p className="text-sm text-muted-foreground">
														{msg.message}
													</p>
												</div>
											))}
										</div>
									</Section>
								)}

							{/* Server Log */}
							<Section title="Server Log">
								<div className="space-y-2">
									<KeyValue label="Log Path" value={supportInfo.log_path} />
									<KeyValue
										label="Log Exists"
										value={supportInfo.log_exists ? "Yes" : "No"}
									/>
									{supportInfo.log_exists && supportInfo.server_log && (
										<div className="mt-2">
											<pre className="text-xs bg-muted p-3 rounded-md overflow-x-auto max-h-60 overflow-y-auto font-mono">
												{supportInfo.server_log}
											</pre>
										</div>
									)}
									{supportInfo.log_exists && !supportInfo.server_log && (
										<p className="text-sm text-muted-foreground">
											Log file is empty
										</p>
									)}
									{!supportInfo.log_exists && (
										<p className="text-sm text-muted-foreground">
											Log file not found
										</p>
									)}
								</div>
							</Section>
						</div>
					)}
				</div>

				<DialogFooter className="gap-2">
					<Button
						variant="outline"
						onClick={handleCopy}
						disabled={!supportInfo}
					>
						<Copy className="h-4 w-4 mr-2" />
						Copy JSON
					</Button>
					<Button
						variant="outline"
						onClick={handleDownload}
						disabled={!supportInfo}
					>
						<Download className="h-4 w-4 mr-2" />
						Download JSON
					</Button>
					<Button variant="default" onClick={onClose}>
						Close
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

function Section({
	title,
	children,
}: {
	title: string;
	children: React.ReactNode;
}) {
	return (
		<div className="space-y-2">
			<h3 className="font-semibold text-sm">{title}</h3>
			<div className="space-y-1.5">{children}</div>
		</div>
	);
}

function KeyValue({ label, value }: { label: string; value: string }) {
	return (
		<div className="flex justify-between items-start gap-4 text-sm">
			<span className="text-muted-foreground shrink-0">{label}:</span>
			<span className="font-mono text-right break-all">{value}</span>
		</div>
	);
}

function formatBytes(bytes: number): string {
	const units = ["B", "KB", "MB", "GB", "TB"];
	let size = bytes;
	let unitIndex = 0;

	while (size >= 1024 && unitIndex < units.length - 1) {
		size /= 1024;
		unitIndex++;
	}

	return `${size.toFixed(2)} ${units[unitIndex]}`;
}
