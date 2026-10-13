import { defineConfig, devices } from '@playwright/test';

const PORT = 5191;

// Golden frames and the audio determinism check run against the built render page (what the
// sidecar loads), in Chromium. Goldens are 480x270 (a quarter of the stage) and shared across
// platforms; the tolerance absorbs font rasterization differences.
export default defineConfig({
	testDir: 'test/golden',
	snapshotPathTemplate: '{testDir}/__goldens__/{arg}{ext}',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
	timeout: 60_000,
	expect: {
		toHaveScreenshot: {
			maxDiffPixelRatio: 0.02,
			threshold: 0.25,
			scale: 'device',
			animations: 'allow',
			caret: 'initial'
		}
	},
	use: {
		...devices['Desktop Chrome'],
		baseURL: `http://localhost:${PORT}`,
		viewport: { width: 1920, height: 1080 },
		deviceScaleFactor: 0.25,
		// as the render sidecar runs it: the full idents collect their canvas reads as they go
		launchOptions: { args: ['--js-flags=--expose-gc'] }
	},
	projects: [{ name: 'chromium' }],
	webServer: {
		command: `npm run build && npx vite preview --port ${PORT} --strictPort`,
		port: PORT,
		reuseExistingServer: !process.env.CI,
		timeout: 120_000
	}
});
