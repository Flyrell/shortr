import type { Locator, Page } from '@playwright/test';

// The browser shares the API container's network namespace, so localhost is both
// the origin under test and a target the redirect can reach.
export const BASE_URL = 'http://localhost:8080';
export const TARGET_URL = `${BASE_URL}/healthz`;
export const SHORT_URL_PATTERN = new RegExp(`^${BASE_URL}/[0-9A-Za-z]{12}$`);
export const COPIED_MESSAGE = 'Copied to clipboard';
export const TOAST_VISIBLE_CLASS = /toast--visible/;
export const INVALID_URL_MESSAGE = 'the url must be an absolute http or https url';

export function tab(page: Page, name: string): Locator {
    return page.getByRole('tab', { name });
}

export function urlInput(page: Page): Locator {
    return page.locator('#url');
}

export function shortUrlButton(page: Page): Locator {
    return page.locator('#result button', { hasText: SHORT_URL_PATTERN });
}

export function toast(page: Page): Locator {
    return page.locator('#toast');
}

export function errorAlert(page: Page): Locator {
    return page.locator('#result [role="alert"]');
}
