import useSWR from "swr";
import { api } from "../api-client";

/**
 * Check if a session status indicates the sandbox is available for requests.
 */
function isSandboxReady(status: string | undefined): boolean {
	return status === "ready" || status === "running";
}

/**
 * Hook for managing services in a session's sandbox.
 * @param sessionId Session ID, or null to disable fetching
 * @param sessionStatus Session status — fetching is paused unless "ready" or "running"
 */
export function useServices(sessionId: string | null, sessionStatus?: string) {
	const shouldFetch = sessionId && isSandboxReady(sessionStatus);

	const { data, error, isLoading, mutate } = useSWR(
		shouldFetch ? `services-${sessionId}` : null,
		() => (sessionId ? api.getServices(sessionId) : null),
		{
			// Poll every 30s normally, 5s when a service is starting
			refreshInterval: (latestData) => {
				const services = latestData?.services || [];
				const hasStarting = services.some((s) => s.status === "starting");
				return hasStarting ? 5000 : 30000;
			},
		},
	);

	/**
	 * Start a service.
	 * @param serviceId Service ID (filename in .discobot/services/)
	 */
	const startService = async (serviceId: string) => {
		if (!sessionId) return;
		await api.startService(sessionId, serviceId);
		mutate();
	};

	/**
	 * Stop a service.
	 * @param serviceId Service ID (filename in .discobot/services/)
	 */
	const stopService = async (serviceId: string) => {
		if (!sessionId) return;
		try {
			await api.stopService(sessionId, serviceId);
		} catch (err) {
			console.error("Failed to stop service:", err);
		} finally {
			mutate();
		}
	};

	/**
	 * Get the URL for streaming service output via SSE.
	 * @param serviceId Service ID
	 */
	const getOutputUrl = (serviceId: string) => {
		if (!sessionId) return null;
		return api.getServiceOutputUrl(sessionId, serviceId);
	};

	return {
		services: data?.services || [],
		isLoading,
		error,
		startService,
		stopService,
		getOutputUrl,
		mutate,
	};
}
