import useSWR from "swr";
import { api } from "../api-client";
import type { EnvSetWithVars } from "../api-types";

/**
 * Hook for managing env sets (named collections of environment variables).
 */
export function useEnvSets() {
	const { data, error, isLoading, mutate } = useSWR("env-sets", () =>
		api.listEnvSets(),
	);

	const createEnvSet = async (
		name: string,
		envVars: Record<string, string>,
	): Promise<EnvSetWithVars> => {
		const envSet = await api.createEnvSet(name, envVars);
		mutate();
		return envSet;
	};

	const updateEnvSet = async (
		id: string,
		name: string,
		envVars: Record<string, string>,
	): Promise<EnvSetWithVars> => {
		const envSet = await api.updateEnvSet(id, name, envVars);
		mutate();
		return envSet;
	};

	const deleteEnvSet = async (id: string): Promise<void> => {
		await api.deleteEnvSet(id);
		mutate();
	};

	return {
		envSets: data?.envSets ?? [],
		isLoading,
		error,
		createEnvSet,
		updateEnvSet,
		deleteEnvSet,
		mutate,
	};
}
