// Files the visitor picked, held by handle for the core, which never holds bytes: an event
// names the handle (`ImageChosen`) and the `upload` effect hands it back to be sent.

const picked = new Map<string, File>();
let last = 0;

/** Keeps `file` until an upload takes it; the handle goes to the core in its place. */
export function holdFile(file: File): string {
	const handle = `file-${++last}`;
	picked.set(handle, file);
	return handle;
}

/** The file behind `handle`, which is let go: each pick is uploaded once. */
export function takeFile(handle: string): File | undefined {
	const file = picked.get(handle);
	picked.delete(handle);
	return file;
}
