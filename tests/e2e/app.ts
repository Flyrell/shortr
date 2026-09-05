import type { Locator, Page } from '@playwright/test';

// The browser shares the API container's network namespace, so localhost is both
// the origin under test and a target the redirect can reach.
export const BASE_URL = 'http://localhost:8080';
export const TARGET_URL = `${BASE_URL}/healthz`;
export const SHORT_URL_PATTERN = new RegExp(`^${BASE_URL}/[0-9A-Za-z]{12}$`);
export const SHORT_CODE_PATTERN = /^[0-9A-Za-z]{12}$/;
export const EXPIRY_PATTERN = /^Expires in \d+ (day|hour|minute)s? \u00B7 \d/;
export const COPIED_MESSAGE = 'Copied to clipboard';
export const TOAST_VISIBLE_CLASS = /(^|\s)on(\s|$)/;
export const INVALID_URL_MESSAGE = 'the url must not contain whitespace';

export function tab(page: Page, name: string): Locator {
    return page.getByRole('tab', { name });
}

export function urlInput(page: Page): Locator {
    return page.locator('#url');
}

export function shortLinkButton(page: Page): Locator {
    return page.locator('#copy-link');
}

export function shortCode(page: Page): Locator {
    return page.locator('#result .link-code');
}

export function expiryLine(page: Page): Locator {
    return page.locator('#result .expiry');
}

export function qrImage(page: Page): Locator {
    return page.locator('#copy-png svg');
}

export function toast(page: Page): Locator {
    return page.locator('#toast');
}

export function errorAlert(page: Page): Locator {
    return page.locator('#error');
}

export function readClipboardText(page: Page): Promise<string> {
    return page.evaluate(() => navigator.clipboard.readText());
}
