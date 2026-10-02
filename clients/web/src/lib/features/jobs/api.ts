// The background job queue plus the admin overview, storage and analytics reads.
import { adminListJobs, type AdminJob } from '$lib/generated/api';

export {
	adminCancelJob,
	adminGetAnalytics,
	adminGetHomeRows,
	adminGetLive,
	adminGetOverview,
	adminGetStorage,
	adminGetSystemStats,
	adminListActiveTranscodes,
	adminListJobs,
	adminRetryJob,
	adminUpdateHomeRows
} from '$lib/generated/api';

export type {
	ActiveTranscode,
	AdminJob,
	AdminJobStatus,
	AnalyticsOverview,
	AnalyticsTopTitle,
	DashboardOverview,
	HomeRowConfig,
	LiveStats,
	StorageCategoryKind,
	StorageInfo,
	SystemStats
} from '$lib/generated/api';

const TERMINAL_JOB_STATUSES = ['done', 'failed', 'cancelled'];

/**
 * Poll the job queue until `id` reaches a terminal status, so callers can
 * refresh once the work has actually landed. Returns the finished job, or null
 * if it didn't settle before the timeout.
 */
export async function waitForJob(
	id: number,
	{ intervalMs = 1500, timeoutMs = 60000 } = {}
): Promise<AdminJob | null> {
	const deadline = Date.now() + timeoutMs;
	while (Date.now() < deadline) {
		const job = (await adminListJobs()).find((j) => j.id === id);
		if (job && TERMINAL_JOB_STATUSES.includes(job.status)) return job;
		await new Promise((resolve) => setTimeout(resolve, intervalMs));
	}
	return null;
}
