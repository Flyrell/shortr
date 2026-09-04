import { expect, test } from '@playwright/test';
import {
    COPIED_MESSAGE,
    EXPIRY_PATTERN,
    errorAlert,
    expiryLine,
    INVALID_URL_MESSAGE,
    readClipboardText,
    SHORT_CODE_PATTERN,
    SHORT_URL_PATTERN,
    shortCode,
    shortLinkButton,
    TARGET_URL,
    TOAST_VISIBLE_CLASS,
    tab,
    toast,
    urlInput,
} from './app';

test('shortens a URL, copies it and redirects to the target', async ({ page, request }) => {
    await page.goto('/');
    await expect(tab(page, 'Shorten URL')).toHaveAttribute('aria-selected', 'true');
    await expect(urlInput(page)).toBeFocused();

    await page.keyboard.type(TARGET_URL);
    await page.keyboard.press('Enter');

    const result = shortLinkButton(page);
    await expect(result).toBeVisible();
    await expect(shortCode(page)).toHaveText(SHORT_CODE_PATTERN);
    await expect(expiryLine(page)).toHaveText(EXPIRY_PATTERN);
    await expect(toast(page)).toHaveText(COPIED_MESSAGE);

    const shortUrl = await readClipboardText(page);
    expect(shortUrl).toMatch(SHORT_URL_PATTERN);

    const redirect = await request.get(shortUrl, { maxRedirects: 0 });
    expect(redirect.status()).toBe(302);
    expect(redirect.headers().location).toBe(TARGET_URL);

    // The toast hides itself; waiting for that is what proves the second copy re-announced it.
    await expect(toast(page)).not.toHaveClass(TOAST_VISIBLE_CLASS);
    await result.click();
    await expect(toast(page)).toHaveText(COPIED_MESSAGE);

    const target = await page.goto(shortUrl);
    if (target === null) {
        throw new Error('e2e: following the short URL produced no response');
    }
    expect(page.url()).toBe(TARGET_URL);
    expect(target.status()).toBe(200);
    expect(await target.text()).toBe('{"status":"ok"}');
});

test('reports a rejected URL in an inline alert', async ({ page }) => {
    await page.goto('/');
    await urlInput(page).fill('not a url');
    await urlInput(page).press('Enter');

    await expect(errorAlert(page)).toContainText(INVALID_URL_MESSAGE);
    await expect(shortLinkButton(page)).toHaveCount(0);
});
