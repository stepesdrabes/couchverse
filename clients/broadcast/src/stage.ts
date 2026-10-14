// The stage: a 1920x1080 box inside a shadow root, so the host page's styles never reach the
// templates and theirs never leak out. Scaled to fit its host (the preview) or 1:1 (renders).

import logoUrl from './assets/logo.svg?url';
import brandCss from './brand/brand.css?inline';
import { parseHex } from './core/palette';
import { colors, radius } from './generated/tokens';
import stageCss from './styles/broadcast.css?inline';
import bulletinCss from './styles/bulletins.css?inline';
import promoCss from './styles/promo.css?inline';
import trailerCss from './styles/trailer.css?inline';

export type Layer = 'picture' | 'matte';

const rgb = (hex: string) => (parseHex(hex) ?? [0, 0, 0]).join(' ');

/** The token values the stylesheets read, as variables on the stage. */
function tokenVars(el: HTMLElement) {
	const vars: Record<string, string> = {
		'--bg': colors.bg,
		'--surface': colors.surface,
		'--surface-2': colors['surface-2'],
		'--edge': colors.edge,
		'--text': colors.text,
		'--muted': colors.muted,
		'--faint': colors.faint,
		'--brand': colors.accent,
		'--brand-strong': colors['accent-strong'],
		'--brand-ink': colors['accent-ink'],
		'--violet': colors['glow-violet'],
		'--teal': colors['glow-teal'],
		'--bg-rgb': rgb(colors.bg),
		'--text-rgb': rgb(colors.text),
		'--surface-2-rgb': rgb(colors['surface-2']),
		'--brand-rgb': rgb(colors.accent),
		'--violet-rgb': rgb(colors['glow-violet']),
		'--teal-rgb': rgb(colors['glow-teal']),
		'--radius-card': `${radius.card}px`,
		'--logo': `url("${logoUrl}")`
	};
	for (const [k, v] of Object.entries(vars)) el.style.setProperty(k, v);
}

export class Stage {
	readonly host: HTMLElement;
	readonly root: ShadowRoot;
	readonly el: HTMLDivElement;
	private readonly resize: ResizeObserver | null = null;

	constructor(host: HTMLElement, { fit = true } = {}) {
		this.host = host;
		this.root = host.shadowRoot ?? host.attachShadow({ mode: 'open' });
		this.root.replaceChildren();
		const style = document.createElement('style');
		style.textContent = `:host { display: block; position: relative; overflow: hidden; background: #000; }\n${stageCss}\n${bulletinCss}\n${promoCss}\n${trailerCss}\n${brandCss}`;
		this.root.appendChild(style);
		this.el = document.createElement('div');
		this.el.className = 'stage';
		tokenVars(this.el);
		this.root.appendChild(this.el);
		if (fit) {
			this.resize = new ResizeObserver(() => this.fit());
			this.resize.observe(host);
			this.fit();
		}
	}

	/** Scale the stage to the host, letterboxed. */
	fit() {
		const w = this.host.clientWidth;
		const h = this.host.clientHeight;
		if (!w || !h) return;
		const k = Math.min(w / 1920, h / 1080);
		this.el.style.transform = `translate(${(w - 1920 * k) / 2}px, ${(h - 1080 * k) / 2}px) scale(${k})`;
	}

	/** A fresh layer for one piece (`hidden`: laid out but never seen, for audio-only builds). */
	layer(hidden = false): HTMLDivElement {
		const l = document.createElement('div');
		l.className = 'piece';
		if (hidden) l.style.visibility = 'hidden';
		this.el.appendChild(l);
		return l;
	}

	setLayer(layer: Layer) {
		this.el.classList.toggle('matte', layer === 'matte');
	}

	destroy() {
		this.resize?.disconnect();
		this.root.replaceChildren();
	}
}
