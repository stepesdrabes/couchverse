import { test as setup, expect } from '@playwright/test';
import { authFile, PASSWORD, USERS } from './fixtures';

// One sign-in per account for the whole run: specs reuse the saved cookie, which also
// keeps them under the login rate limit (five attempts a minute per account).
for (const user of USERS) {
	setup(`sign in as ${user}`, async ({ request }) => {
		const res = await request.post('/api/v1/auth/login', {
			data: { username: user, password: PASSWORD }
		});
		expect(res.ok()).toBe(true);
		await request.storageState({ path: authFile(user) });
	});
}
