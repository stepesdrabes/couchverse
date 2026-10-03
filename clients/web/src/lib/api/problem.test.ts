import { describe, expect, it, vi } from 'vitest';
import { ApiError } from './client';
import { problemMessage } from './problem';

// the compiled catalogs and the display language need a browser; stand-ins replace them
vi.mock('$lib/i18n/locale.svelte', () => ({ currentLang: () => 'en' }));
vi.mock('$lib/paraglide/messages', () => ({
	problem_invalid_credentials: () => 'Wrong username or password.',
	problem_offline: () => "Can't reach the server."
}));

describe('problemMessage', () => {
	it('names a documented error code in the display language', () => {
		const err = new ApiError(401, 'invalid_credentials', 'invalid username or password');
		expect(problemMessage(err, 'fallback')).toBe('Wrong username or password.');
	});

	it('falls back for codes without a message, never showing the server text', () => {
		const err = new ApiError(500, 'internal', 'internal server error');
		expect(problemMessage(err, 'fallback')).toBe('fallback');
	});

	it('says the server is out of reach when no response came back', () => {
		expect(problemMessage(new TypeError('Failed to fetch'), 'fallback')).toBe(
			"Can't reach the server."
		);
	});
});
