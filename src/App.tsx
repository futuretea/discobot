import { Analytics } from "@vercel/analytics/react";
import { Route, Routes } from "react-router";
import { AppShell } from "@/components/app-shell";
import { ResizeObserverFix } from "@/components/resize-observer-fix";
import { ThemeProvider } from "@/components/theme-provider";
import { HomePage } from "./pages/HomePage";

export function App() {
	return (
		<ThemeProvider
			attribute="class"
			defaultTheme="dark"
			enableSystem
			disableTransitionOnChange
			storageKey="theme"
		>
			<ResizeObserverFix />
			<AppShell>
				<Routes>
					<Route path="/" element={<HomePage />} />
				</Routes>
			</AppShell>
			<Analytics />
		</ThemeProvider>
	);
}
