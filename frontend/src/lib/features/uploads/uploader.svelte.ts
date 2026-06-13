import * as uploadsApi from './api';
import type { LibraryKind, UploadAssign, UploadSession } from './api';

const CHUNK_SIZE = 8 * 1024 * 1024;

export type UploadStatus = 'queued' | 'uploading' | 'paused' | 'completing' | 'done' | 'error';

export interface UploadOpts {
	assign?: UploadAssign;
	onDone?: () => void;
}

export class Upload {
	readonly file: File;
	readonly libraryKind: LibraryKind;
	readonly opts: UploadOpts;
	sessionId = $state<string | null>(null);
	offset = $state(0);
	status = $state<UploadStatus>('queued');
	error = $state('');

	private aborter: AbortController | null = null;

	constructor(
		file: File,
		libraryKind: LibraryKind,
		opts: UploadOpts = {},
		resumeFrom?: UploadSession
	) {
		this.file = file;
		this.libraryKind = libraryKind;
		this.opts = opts;
		if (resumeFrom) {
			this.sessionId = resumeFrom.id;
			this.offset = resumeFrom.receivedBytes;
		}
	}

	get progress() {
		return this.file.size > 0 ? this.offset / this.file.size : 0;
	}

	async start() {
		if (this.status === 'uploading' || this.status === 'done') return;
		this.status = 'uploading';
		this.error = '';
		this.aborter = new AbortController();

		try {
			if (!this.sessionId) {
				const session = await uploadsApi.createSession(this.file.name, this.file.size);
				this.sessionId = session.id;
				this.offset = session.receivedBytes;
			} else {
				// resync with the server after a pause/reload
				const session = await uploadsApi.getSession(this.sessionId);
				this.offset = session.receivedBytes;
			}

			while (this.offset < this.file.size) {
				if (this.aborter.signal.aborted) return;
				const chunk = this.file.slice(this.offset, this.offset + CHUNK_SIZE);
				this.offset = await uploadsApi.putChunk(
					this.sessionId,
					this.offset,
					chunk,
					this.aborter.signal
				);
			}

			this.status = 'completing';
			await uploadsApi.completeSession(this.sessionId, this.libraryKind, this.opts.assign);
			this.status = 'done';
			this.opts.onDone?.();
		} catch (err) {
			if (this.aborter?.signal.aborted) return; // paused, not an error
			this.status = 'error';
			this.error = err instanceof Error ? err.message : 'upload failed';
		}
	}

	pause() {
		if (this.status !== 'uploading') return;
		this.aborter?.abort();
		this.status = 'paused';
	}

	async abort() {
		this.aborter?.abort();
		if (this.sessionId) {
			try {
				await uploadsApi.abortSession(this.sessionId);
			} catch {
				// already gone
			}
		}
		this.status = 'error';
		this.error = 'cancelled';
	}
}

export class UploadQueue {
	uploads = $state<Upload[]>([]);

	/**
	 * Queue files for upload, returning the created entries. Files matching an
	 * interrupted server session (same name + size) resume from its offset.
	 */
	async add(files: File[], libraryKind: LibraryKind, opts: UploadOpts = {}): Promise<Upload[]> {
		let sessions: UploadSession[] = [];
		try {
			sessions = await uploadsApi.listSessions();
		} catch {
			// non-fatal - uploads just start fresh
		}

		const created: Upload[] = [];
		for (const file of files) {
			const resume = sessions.find(
				(s) => s.filename === file.name && s.declaredSize === file.size && s.status === 'active'
			);
			const upload = new Upload(file, libraryKind, opts, resume);
			created.push(upload);
			this.uploads = [...this.uploads, upload];
			upload.start();
		}
		return created;
	}

	remove(upload: Upload) {
		this.uploads = this.uploads.filter((u) => u !== upload);
	}

	/**
	 * True while losing the tab would interrupt or discard upload progress -
	 * drives the beforeunload guard. Paused/errored uploads with bytes already
	 * sent count too, since a reload restarts them from scratch.
	 */
	get inFlight(): boolean {
		return this.uploads.some(
			(u) =>
				u.status === 'uploading' ||
				u.status === 'completing' ||
				u.status === 'queued' ||
				((u.status === 'paused' || u.status === 'error') && u.offset > 0)
		);
	}
}

export const uploadQueue = new UploadQueue();
