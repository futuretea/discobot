import { useTheme } from "next-themes";
import { useCallback, useEffect, useState } from "react";
import type { ThemeColorScheme } from "@/lib/api-types";
import { THEMES } from "@/lib/theme-constants";
import { PREFERENCE_KEYS, usePreferences } from "./use-preferences";

export function useThemeCustomization() {
	const { theme, setTheme, resolvedTheme } = useTheme();
	const { getPreference, setPreference } = usePreferences();
	const [colorScheme, setColorSchemeState] =
		useState<ThemeColorScheme>("default");
	const [mounted, setMounted] = useState(false);

	const applyThemeAttribute = useCallback((scheme: ThemeColorScheme) => {
		if (typeof document !== "undefined") {
			document.documentElement.setAttribute("data-theme", scheme);
		}
	}, []);

	// Load from preferences on mount and when resolved theme changes
	useEffect(() => {
		setMounted(true);
		if (!resolvedTheme) return;

		// Get the preference key for the current mode (light or dark)
		const preferenceKey =
			resolvedTheme === "light"
				? PREFERENCE_KEYS.THEME_COLOR_SCHEME_LIGHT
				: PREFERENCE_KEYS.THEME_COLOR_SCHEME_DARK;
		const saved = getPreference(preferenceKey) as ThemeColorScheme | undefined;

		if (saved) {
			setColorSchemeState(saved);
			applyThemeAttribute(saved);
		} else {
			// Apply default theme for this mode
			setColorSchemeState("default");
			applyThemeAttribute("default");
		}
	}, [getPreference, applyThemeAttribute, resolvedTheme]);

	const setColorScheme = (scheme: ThemeColorScheme) => {
		setColorSchemeState(scheme);
		applyThemeAttribute(scheme);

		// Save to the preference for the current mode
		if (resolvedTheme) {
			const preferenceKey =
				resolvedTheme === "light"
					? PREFERENCE_KEYS.THEME_COLOR_SCHEME_LIGHT
					: PREFERENCE_KEYS.THEME_COLOR_SCHEME_DARK;
			setPreference(preferenceKey, scheme);
		}
	};

	// Filter themes based on current light/dark mode
	const availableThemes = THEMES.filter((t) => t.mode === resolvedTheme);

	return {
		theme,
		setTheme,
		resolvedTheme,
		colorScheme,
		setColorScheme,
		availableThemes,
		mounted,
	};
}
