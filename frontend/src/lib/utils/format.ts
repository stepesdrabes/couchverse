export function formatBytes(bytes: number): string {
	if (bytes <= 0) return '—';
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	let i = 0;
	let value = bytes;
	while (value >= 1024 && i < units.length - 1) {
		value /= 1024;
		i++;
	}
	return `${value >= 100 ? Math.round(value) : value.toFixed(1)} ${units[i]}`;
}

export function formatDate(iso: string): string {
	return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export function formatYearDate(iso: string): string {
	return new Date(iso).toLocaleDateString(undefined, {
		year: 'numeric',
		month: 'short',
		day: 'numeric'
	});
}

/** 95 → "1h 35m", 47 → "47m" */
export function formatRuntime(minutes: number): string {
	if (minutes < 60) return `${minutes}m`;
	return `${Math.floor(minutes / 60)}h ${minutes % 60 ? `${minutes % 60}m` : ''}`.trim();
}

/** 754 → "12:34" */
export function formatClock(totalSeconds: number): string {
	const s = Math.max(0, Math.floor(totalSeconds));
	const h = Math.floor(s / 3600);
	const m = Math.floor((s % 3600) / 60);
	const sec = `${s % 60}`.padStart(2, '0');
	return h > 0 ? `${h}:${`${m}`.padStart(2, '0')}:${sec}` : `${m}:${sec}`;
}

/** 3725 → "1h 2m", 45 → "45s", 90061 → "1d 1h" */
export function formatUptime(totalSeconds: number): string {
	const s = Math.max(0, Math.floor(totalSeconds));
	const d = Math.floor(s / 86400);
	const h = Math.floor((s % 86400) / 3600);
	const m = Math.floor((s % 3600) / 60);
	if (d > 0) return `${d}d ${h}h`;
	if (h > 0) return `${h}h ${m}m`;
	if (m > 0) return `${m}m`;
	return `${s}s`;
}

/** media file height → quality badge label */
export function qualityLabel(height: number): string | null {
	if (height >= 2000) return '4K';
	if (height >= 1000) return '1080P';
	if (height >= 700) return '720P';
	if (height > 0) return 'SD';
	return null;
}
