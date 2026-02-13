import { Info, Key, Settings } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { isTauri } from "@/lib/api-config";
import { useDialogContext } from "@/lib/contexts/dialog-context";
import { useMainContentContext } from "@/lib/contexts/main-content-context";
import { useThemeCustomization } from "@/lib/hooks/use-theme-customization";

interface SettingsMenuProps {
	className?: string;
	isMac?: boolean;
}

export function SettingsMenu({ className, isMac = false }: SettingsMenuProps) {
	const dialogs = useDialogContext();
	const { chatWidthMode, setChatWidthMode } = useMainContentContext();

	// Theme customization
	const {
		theme,
		setTheme,
		colorScheme,
		setColorScheme,
		availableThemes,
		mounted: themeMounted,
	} = useThemeCustomization();

	return (
		<DropdownMenu>
			<DropdownMenuTrigger asChild>
				<Button
					variant="ghost"
					size="icon"
					title="Settings"
					className={className}
				>
					<Settings className="h-4 w-4" />
					<span className="sr-only">Settings</span>
				</Button>
			</DropdownMenuTrigger>
			<DropdownMenuContent align="end" className="w-80">
				{/* Hide theme controls on macOS Tauri (follows system theme) */}
				{!(isTauri() && isMac) && themeMounted && (
					<>
						<DropdownMenuLabel>Appearance</DropdownMenuLabel>
						<div className="px-2 py-2 space-y-4">
							{/* Mode selector */}
							<div className="space-y-2">
								<Label className="text-xs text-muted-foreground">Mode</Label>
								<RadioGroup
									value={theme}
									onValueChange={setTheme}
									className="flex gap-2"
								>
									<div className="flex-1">
										<RadioGroupItem
											value="light"
											id="mode-light"
											className="peer sr-only"
										/>
										<Label
											htmlFor="mode-light"
											className="flex items-center justify-center rounded-md border-2 border-muted bg-transparent px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground peer-data-[state=checked]:border-primary peer-data-[state=checked]:bg-accent"
										>
											Light
										</Label>
									</div>
									<div className="flex-1">
										<RadioGroupItem
											value="dark"
											id="mode-dark"
											className="peer sr-only"
										/>
										<Label
											htmlFor="mode-dark"
											className="flex items-center justify-center rounded-md border-2 border-muted bg-transparent px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground peer-data-[state=checked]:border-primary peer-data-[state=checked]:bg-accent"
										>
											Dark
										</Label>
									</div>
									<div className="flex-1">
										<RadioGroupItem
											value="system"
											id="mode-system"
											className="peer sr-only"
										/>
										<Label
											htmlFor="mode-system"
											className="flex items-center justify-center rounded-md border-2 border-muted bg-transparent px-3 py-2 text-sm cursor-pointer hover:bg-accent hover:text-accent-foreground peer-data-[state=checked]:border-primary peer-data-[state=checked]:bg-accent"
										>
											System
										</Label>
									</div>
								</RadioGroup>
							</div>

							{/* Theme selector */}
							<div className="space-y-2">
								<Label className="text-xs text-muted-foreground">Theme</Label>
								<Select value={colorScheme} onValueChange={setColorScheme}>
									<SelectTrigger className="w-full">
										<SelectValue>
											{availableThemes.find((t) => t.id === colorScheme) && (
												<div className="flex items-center gap-2">
													<div className="flex gap-1">
														<div
															className="w-3 h-3 rounded border border-border"
															style={{
																background: availableThemes.find(
																	(t) => t.id === colorScheme,
																)?.preview.background,
															}}
														/>
														<div
															className="w-3 h-3 rounded border border-border"
															style={{
																background: availableThemes.find(
																	(t) => t.id === colorScheme,
																)?.preview.primary,
															}}
														/>
														<div
															className="w-3 h-3 rounded border border-border"
															style={{
																background: availableThemes.find(
																	(t) => t.id === colorScheme,
																)?.preview.foreground,
															}}
														/>
													</div>
													<span>
														{
															availableThemes.find((t) => t.id === colorScheme)
																?.name
														}
													</span>
												</div>
											)}
										</SelectValue>
									</SelectTrigger>
									<SelectContent>
										{availableThemes.map((themeOption) => (
											<SelectItem key={themeOption.id} value={themeOption.id}>
												<div className="flex items-center gap-2">
													<div className="flex gap-1">
														<div
															className="w-3 h-3 rounded border border-border"
															style={{
																background: themeOption.preview.background,
															}}
														/>
														<div
															className="w-3 h-3 rounded border border-border"
															style={{
																background: themeOption.preview.primary,
															}}
														/>
														<div
															className="w-3 h-3 rounded border border-border"
															style={{
																background: themeOption.preview.foreground,
															}}
														/>
													</div>
													<span>{themeOption.name}</span>
												</div>
											</SelectItem>
										))}
									</SelectContent>
								</Select>
							</div>
						</div>
						<DropdownMenuSeparator />
					</>
				)}

				<DropdownMenuLabel>Chat</DropdownMenuLabel>
				<div className="flex items-center justify-between px-2 py-2">
					<Label
						htmlFor="chat-full-width-toggle"
						className="text-sm font-normal cursor-pointer"
					>
						Full width
					</Label>
					<Switch
						id="chat-full-width-toggle"
						checked={chatWidthMode === "full"}
						onCheckedChange={(checked) =>
							setChatWidthMode(checked ? "full" : "constrained")
						}
					/>
				</div>

				<DropdownMenuSeparator />
				<DropdownMenuItem onClick={() => dialogs.credentialsDialog.open()}>
					<Key className="h-4 w-4 mr-2" />
					API Credentials
				</DropdownMenuItem>
				<DropdownMenuItem onClick={() => dialogs.supportInfoDialog.open()}>
					<Info className="h-4 w-4 mr-2" />
					Support Information
				</DropdownMenuItem>
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
