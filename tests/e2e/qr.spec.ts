import { expect, test } from '@playwright/test';
import { COPIED_MESSAGE, SHORT_URL_PATTERN, shortUrlButton, TARGET_URL, tab, toast, urlInput } from './app';

test('creates a QR code, copies the PNG and remembers the mode', async ({ page }) => {
    await page.goto('/');
    await tab(page, 'Shorten URL').focus();
    await page.keyboard.press('ArrowRight');

    const qrTab = tab(page, 'QR code');
    await expect(qrTab).toHaveAttribute('aria-selected', 'true');

    await urlInput(page).fill(TARGET_URL);
    await urlInput(page).press('Enter');

    await expect(page.locator('#result img[alt^="QR code"]')).toBeVisible();
    await expect(toast(page)).toHaveText(COPIED_MESSAGE);

    const types = await page.evaluate(async () => {
        const items = await navigator.clipboard.read();
        return items.flatMap((item) => item.types);
    });
    expect(types).toContain('image/png');

    const link = shortUrlButton(page);
    await expect(link).toHaveText(SHORT_URL_PATTERN);
    await link.click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toMatch(SHORT_URL_PATTERN);

    await page.reload();
    await expect(qrTab).toHaveAttribute('aria-selected', 'true');

    await qrTab.focus();
    await page.keyboard.press('ArrowLeft');
    await page.reload();
    await expect(tab(page, 'Shorten URL')).toHaveAttribute('aria-selected', 'true');
});
