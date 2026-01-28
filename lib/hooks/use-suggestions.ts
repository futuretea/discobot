"use client";

import { useEffect, useState } from "react";
import useSWR from "swr";
import { api } from "../api-client";

export function useSuggestions(query: string, type?: "path" | "repo") {
	const [debouncedQuery, setDebouncedQuery] = useState(query);

	// Debounce the query - wait 150ms after user stops typing
	useEffect(() => {
		const timer = setTimeout(() => {
			setDebouncedQuery(query);
		}, 150);

		return () => clearTimeout(timer);
	}, [query]);

	const { data, error, isLoading } = useSWR(
		debouncedQuery.length >= 1 ? `suggestions-${debouncedQuery}-${type}` : null,
		() => api.getSuggestions(debouncedQuery, type),
		{ dedupingInterval: 300 },
	);

	return {
		suggestions: data?.suggestions || [],
		isLoading,
		error,
	};
}
