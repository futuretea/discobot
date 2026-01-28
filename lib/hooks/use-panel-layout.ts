
import * as React from "react";
import { STORAGE_KEYS } from "./use-persisted-state";

export type PanelState = "normal" | "minimized" | "maximized";

/**
 * Load persisted panel state from localStorage
 */
function loadPersistedState(): Partial<PanelLayoutState> {
	if (typeof window === "undefined") return {};

	try {
		const bottomPanelState = localStorage.getItem(
			STORAGE_KEYS.BOTTOM_PANEL_STATE,
		);

		return {
			...(bottomPanelState && {
				bottomPanelState: JSON.parse(bottomPanelState),
			}),
		};
	} catch {
		return {};
	}
}

/**
 * Save panel state to localStorage
 */
function savePersistedState(state: PanelLayoutState): void {
	if (typeof window === "undefined") return;

	try {
		localStorage.setItem(
			STORAGE_KEYS.BOTTOM_PANEL_STATE,
			JSON.stringify(state.bottomPanelState),
		);
	} catch {
		// Ignore storage errors
	}
}

type PanelAction =
	| { type: "INIT"; persisted: Partial<PanelLayoutState> }
	| { type: "MINIMIZE_BOTTOM" }
	| { type: "MAXIMIZE_BOTTOM" }
	| { type: "RESET" };

interface PanelLayoutState {
	bottomPanelState: PanelState;
}

function panelReducer(
	state: PanelLayoutState,
	action: PanelAction,
): PanelLayoutState {
	switch (action.type) {
		case "INIT":
			return { ...state, ...action.persisted };

		case "MINIMIZE_BOTTOM":
			if (state.bottomPanelState === "minimized") {
				return {
					...state,
					bottomPanelState: "normal",
				};
			}
			return {
				...state,
				bottomPanelState: "minimized",
			};

		case "MAXIMIZE_BOTTOM":
			if (state.bottomPanelState === "maximized") {
				return {
					...state,
					bottomPanelState: "normal",
				};
			}
			return {
				...state,
				bottomPanelState: "maximized",
			};

		case "RESET":
			return { ...state, bottomPanelState: "normal" };

		default:
			return state;
	}
}

export function usePanelLayout() {
	const [state, dispatch] = React.useReducer(panelReducer, {
		bottomPanelState: "normal",
	});

	const mainRef = React.useRef<HTMLDivElement>(null);

	// Load persisted state on mount
	React.useEffect(() => {
		const persisted = loadPersistedState();
		if (Object.keys(persisted).length > 0) {
			dispatch({ type: "INIT", persisted });
		}
	}, []);

	// Save state changes to localStorage
	React.useEffect(() => {
		savePersistedState(state);
	}, [state]);

	const getBottomPanelStyle = React.useCallback((): React.CSSProperties => {
		if (state.bottomPanelState === "minimized") return { height: 40 };
		if (state.bottomPanelState === "maximized") return { flex: 1 };
		return { flex: 1 };
	}, [state.bottomPanelState]);

	// Memoize action handlers
	const handleBottomMinimize = React.useCallback(
		() => dispatch({ type: "MINIMIZE_BOTTOM" }),
		[],
	);
	const handleBottomMaximize = React.useCallback(
		() => dispatch({ type: "MAXIMIZE_BOTTOM" }),
		[],
	);
	const resetPanels = React.useCallback(() => dispatch({ type: "RESET" }), []);

	return {
		// State
		bottomPanelState: state.bottomPanelState,
		mainRef,

		// Styles
		getBottomPanelStyle,

		// Actions
		handleBottomMinimize,
		handleBottomMaximize,
		resetPanels,
	};
}
