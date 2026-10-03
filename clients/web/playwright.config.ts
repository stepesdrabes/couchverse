import { defineConfig, devices } from '@playwright/test';

const ci = !!process.env.CI;

export default defineConfig({
	testDir: 'e2e',
	fullyParallel: true,
	forbidOnly: ci,
	retries: ci ? 1 : 0,
	reporter: [[ci ? 'github' : 'list'], ['html', { open: 'never' }]],
	// shared CI runners render without a GPU and can be several times slower
	timeout: ci ? 60_000 : 30_000,
	expect: { timeout: ci ? 10_000 : 5_000 },
	use: {
		// captured from the server's ready line below; the config is re-read in each
		// worker, after the server has started
		baseURL: process.env.E2E_BASE_URL,
		locale: 'en-US',
		trace: 'retain-on-failure',
		screenshot: 'only-on-failure'
	},
	projects: [
		{ name: 'setup', testMatch: /\.setup\.ts$/ },
		{ name: 'chromium', use: { ...devices['Desktop Chrome'] }, dependencies: ['setup'] },
		// after Chromium, not beside it: both drive the same accounts on one server. CI runs
		// Chromium only until WebKit is proven on Linux runners (it plays the clip on macOS).
		{ name: 'webkit', use: { ...devices['Desktop Safari'] }, dependencies: ['chromium'] }
	],
	webServer: {
		command: '../../scripts/e2e-server.sh',
		wait: { stdout: /e2e server ready at (?<e2e_base_url>http:\/\/\S+)/ },
		// covers `make build` on a cold cache
		timeout: 5 * 60_000,
		// lets the script drop its database and temp dir
		gracefulShutdown: { signal: 'SIGTERM', timeout: 15_000 }
	}
});
