<script lang="ts">
	import { encode } from 'uqr';

	let {
		value,
		label,
		size = 224,
		class: cls = ''
	}: {
		value: string;
		label: string;
		/** largest rendered width in px; it snaps down to whole pixels per module */
		size?: number;
		class?: string;
	} = $props();

	// Scanners need a light quiet zone around the code to find it, and the dark UI
	// around it does not count, so the border is part of the image.
	const QUIET_ZONE = 4;

	const qr = $derived(encode(value, { ecc: 'M', border: QUIET_ZONE }));
	const width = $derived(qr.size * Math.max(1, Math.floor(size / qr.size)));

	// one path of horizontal runs keeps the SVG small and the module edges seamless
	const path = $derived.by(() => {
		let d = '';
		qr.data.forEach((row, y) => {
			let x = 0;
			while (x < row.length) {
				if (!row[x]) {
					x++;
					continue;
				}
				const start = x;
				while (x < row.length && row[x]) x++;
				d += `M${start} ${y}h${x - start}v1h${start - x}z`;
			}
		});
		return d;
	});
</script>

<svg
	viewBox="0 0 {qr.size} {qr.size}"
	{width}
	height={width}
	role="img"
	aria-label={label}
	shape-rendering="crispEdges"
	class="block {cls}"
>
	<rect width={qr.size} height={qr.size} fill="#fff" />
	<path d={path} fill="#000" />
</svg>
