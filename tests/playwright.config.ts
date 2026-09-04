import { defineConfig, devices } from '@playwright/test';
import { BASE_URL } from './e2e/app';

const desktopChrome = devices['Desktop Chrome'];
if (desktopChrome === undefined) {
    throw new Error('playwright: the Desktop Chrome device descriptor is missing');
}

export default defineConfig({
    testDir: './e2e',
    timeout: 30_000,
    retries: 0,
    reporter: 'list',
    use: {
        baseURL: BASE_URL,
        permissions: ['clipboard-read', 'clipboard-write'],
    },
    projects: [{ name: 'chromium', use: desktopChrome }],
});
