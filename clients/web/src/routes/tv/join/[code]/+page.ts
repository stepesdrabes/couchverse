// Public: anyone with the channel link joins under a name they type; the code is the link's.
export function load({ params }) {
	return { code: params.code };
}
