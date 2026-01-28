import "./globals.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import { App } from "./App";

const reactDevToolsUrl = import.meta.env.VITE_REACT_DEVTOOLS_URL;

// Load React DevTools if URL is provided
if (reactDevToolsUrl) {
	const script = document.createElement("script");
	script.src = reactDevToolsUrl;
	script.async = true;
	document.head.appendChild(script);
}

const root = document.getElementById("root");
if (!root) throw new Error("Root element not found");

createRoot(root).render(
	<StrictMode>
		<BrowserRouter>
			<App />
		</BrowserRouter>
	</StrictMode>,
);
