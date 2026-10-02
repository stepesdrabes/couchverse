export {
	adminAbortUpload,
	adminCompleteUpload,
	adminCreateUpload,
	adminGetUpload,
	adminListUploads
} from '$lib/generated/api';
export type {
	UploadAssignment,
	UploadAssignmentLibraryKind,
	UploadSession
} from '$lib/generated/api';

/** raw chunk PUT - returns the server offset after the append */
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
