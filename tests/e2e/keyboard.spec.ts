import { expect, test } from '@playwright/test';
import {
    COPIED_MESSAGE,
    SHORT_URL_PATTERN,
    shortUrlButton,
    TARGET_URL,
    TOAST_VISIBLE_CLASS,
    tab,
    toast,
    urlInput,
} from './app';

test('completes the shorten flow with the keyboard alone', async ({ page }) => {
    await page.goto('/');
    await page.keyboard.press('Shift+Tab');
    await expect(tab(page, 'Shorten URL')).toBeFocused();

    await page.keyboard.press('Tab');
    await expect(urlInput(page)).toBeFocused();
    await page.keyboard.type(TARGET_URL);
    await page.keyboard.press('Enter');

    const result = shortUrlButton(page);
    await expect(result).toHaveText(SHORT_URL_PATTERN);
    await expect(toast(page)).toHaveText(COPIED_MESSAGE);
    await expect(toast(page)).not.toHaveClass(TOAST_VISIBLE_CLASS);

    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await expect(result).toBeFocused();

    await page.keyboard.press('Enter');
    await expect(toast(page)).toHaveText(COPIED_MESSAGE);
});
