import { api } from '$lib/api/client';

export interface UploadSession {
	id: string;
	userId: number;
	filename: string;
	declaredSize: number;
	receivedBytes: number;
	status: 'active' | 'complete' | 'aborted';
	createdAt: string;
	updatedAt: string;
	expiresAt: string;
}

export type LibraryKind = 'movies' | 'series' | 'music';

export const listSessions = () => api<UploadSession[]>('/admin/uploads');

export const createSession = (filename: string, size: number) =>
	api<UploadSession>('/admin/uploads', { method: 'POST', body: { filename, size } });

export const getSession = (id: string) => api<UploadSession>(`/admin/uploads/${id}`);

export interface UploadAssign {
	/** attach the upload to a specific movie title or series (episode resolved from SxxExx) */
	titleId?: number;
}

export const completeSession = (id: string, libraryKind: LibraryKind, assign: UploadAssign = {}) =>
	api<{ mediaFileId: number }>(`/admin/uploads/${id}/complete`, {
		method: 'POST',
		body: { libraryKind, ...assign }
	});

export const abortSession = (id: string) => api<void>(`/admin/uploads/${id}`, { method: 'DELETE' });

/** raw chunk PUT — returns the server offset after the append */
export async function putChunk(
	id: string,
	offset: number,
	chunk: Blob,
	signal: AbortSignal
): Promise<number> {
	const res = await fetch(`/api/v1/admin/uploads/${id}?offset=${offset}`, {
		method: 'PUT',
		body: chunk,
		credentials: 'same-origin',
		signal
	});
	const data = await res.json();
	if (res.status === 409) return data.offset; // resync and continue
	if (!res.ok) throw new Error(data.error?.message ?? 'chunk upload failed');
	return data.offset;
}
