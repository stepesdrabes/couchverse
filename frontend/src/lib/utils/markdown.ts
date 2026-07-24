import MarkdownIt from 'markdown-it';

/**
 * The one markdown instance, configured for user-authored content.
 *
 * `html: false` is the security boundary: raw HTML in a bio is escaped rather
 * than parsed, so a member cannot inject markup into someone else's page.
 * markdown-it also validates link protocols itself, rejecting `javascript:` and
 * friends, which is why no separate sanitizer is needed on top.
 */
const md = new MarkdownIt('default', {
	html: false,
	linkify: true,
	// a single newline is a line break: people write bios like chat messages,
	// not like documents
	breaks: true,
	typographer: true
});

// Headings above h3 would out-shout the page's own headings, and images would
// let a bio smuggle in arbitrary remote requests, so neither is enabled.
md.disable(['image']);

// Every link leaves to somewhere untrusted.
const defaultLinkOpen =
	md.renderer.rules.link_open ??
	((tokens, i, options, _env, self) => self.renderToken(tokens, i, options));

md.renderer.rules.link_open = (tokens, i, options, env, self) => {
	tokens[i].attrSet('target', '_blank');
	tokens[i].attrSet('rel', 'nofollow noopener noreferrer');
	return defaultLinkOpen(tokens, i, options, env, self);
};

export const renderMarkdown = (source: string) => md.render(source);

/** plain-text preview for places with no room for formatting (admin tables) */
export const stripMarkdown = (source: string) =>
	source
		.replace(/[#>*_`~\-[\]()]/g, ' ')
		.replace(/\s+/g, ' ')
		.trim();
