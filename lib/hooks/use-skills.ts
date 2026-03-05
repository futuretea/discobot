import useSWR from "swr";
import { api } from "../api-client";
import type {
	Skill,
	CreateSkillRequest,
	UpdateSkillRequest,
	ImportSkillRequest,
	SkillCatalogEntry,
	SkillMarketEntry,
	SkillMarketRepo,
	CreateSkillMarketRepoRequest,
	UpdateSkillMarketRepoRequest,
} from "../api-types";

export function useSkills() {
	const { data, error, isLoading, mutate } = useSWR("skills", () =>
		api.getSkills(),
	);

	const createSkill = async (payload: CreateSkillRequest) => {
		const skill = await api.createSkill(payload);
		mutate();
		return skill;
	};

	const updateSkill = async (id: string, payload: UpdateSkillRequest) => {
		const skill = await api.updateSkill(id, payload);
		mutate();
		return skill;
	};

	const deleteSkill = async (id: string) => {
		await api.deleteSkill(id);
		mutate();
	};

	const importSkill = async (payload: ImportSkillRequest) => {
		const skill = await api.importSkill(payload);
		mutate();
		return skill;
	};

	return {
		skills: (data?.skills || []) as Skill[],
		isLoading,
		error,
		createSkill,
		updateSkill,
		deleteSkill,
		importSkill,
		mutate,
	};
}

export function useSkillCatalog() {
	const { data, error, isLoading, mutate } = useSWR(
		"skill-catalog",
		() => api.getSkillCatalog(),
	);

	return {
		catalog: (data?.skills || []) as SkillCatalogEntry[],
		isLoading,
		error,
		mutate,
	};
}

export function useSkillMarket(repoUrl?: string, branch?: string, skillsPath?: string) {
	// Only fetch if repoUrl is non-empty string or undefined (use server default)
	const key = repoUrl !== undefined
		? (repoUrl ? ["skill-market", repoUrl, branch ?? "", skillsPath ?? ""] : null)
		: "skill-market";
	const { data, error, isLoading, mutate } = useSWR(
		key,
		() => api.getSkillMarket(repoUrl || undefined, branch || undefined, skillsPath || undefined),
	);

	// reload forces a "git pull" on the server before returning the updated list.
	const reload = (): Promise<{ skills: SkillMarketEntry[] } | undefined> =>
		mutate(
			api.getSkillMarket(repoUrl || undefined, branch || undefined, skillsPath || undefined, true),
		);

	return {
		skills: (data?.skills || []) as SkillMarketEntry[],
		isLoading,
		error,
		mutate,
		reload,
	};
}

export function useSkillMarketRepos() {
	const { data, error, isLoading, mutate } = useSWR("skill-market-repos", () =>
		api.getSkillMarketRepos(),
	);

	const createRepo = async (payload: CreateSkillMarketRepoRequest) => {
		const repo = await api.createSkillMarketRepo(payload);
		mutate();
		return repo;
	};

	const updateRepo = async (id: string, payload: UpdateSkillMarketRepoRequest) => {
		const repo = await api.updateSkillMarketRepo(id, payload);
		mutate();
		return repo;
	};

	const deleteRepo = async (id: string) => {
		await api.deleteSkillMarketRepo(id);
		mutate();
	};

	return {
		repos: (data?.repos || []) as SkillMarketRepo[],
		isLoading,
		error,
		createRepo,
		updateRepo,
		deleteRepo,
		mutate,
	};
}
