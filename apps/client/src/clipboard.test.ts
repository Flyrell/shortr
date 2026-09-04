import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { copyPng, copyText } from './clipboard';

const writeText = vi.fn<(text: string) => Promise<void>>();
const write = vi.fn<(items: unknown[]) => Promise<void>>();

const originalClipboard = Object.getOwnPropertyDescriptor(globalThis.navigator, 'clipboard');

function setClipboard(value: unknown): void {
    Object.defineProperty(globalThis.navigator, 'clipboard', { value, configurable: true, writable: true });
}

function restoreClipboard(): void {
    if (originalClipboard === undefined) {
        Reflect.deleteProperty(globalThis.navigator, 'clipboard');
        return;
    }
    Object.defineProperty(globalThis.navigator, 'clipboard', originalClipboard);
}

function pngOf(item: unknown): unknown {
    if (typeof item !== 'object' || item === null || !('items' in item)) {
        return null;
    }
    const entries = item.items;
    if (typeof entries !== 'object' || entries === null || !('image/png' in entries)) {
        return null;
    }
    return entries['image/png'];
}

beforeEach(() => {
    writeText.mockReset().mockResolvedValue(undefined);
    write.mockReset().mockResolvedValue(undefined);
    setClipboard({ writeText, write });
    vi.stubGlobal(
        'ClipboardItem',
        class {
            readonly items: Record<string, Blob>;
            constructor(items: Record<string, Blob>) {
                this.items = items;
            }
        },
    );
});

afterEach(() => {
    restoreClipboard();
    vi.unstubAllGlobals();
});

describe('copyText', () => {
    test('writes the text and reports success', async () => {
        expect(await copyText('https://s.io/abc1234')).toBe(true);
        expect(writeText).toHaveBeenCalledWith('https://s.io/abc1234');
    });

    test('reports failure when the clipboard API is missing', async () => {
        setClipboard(undefined);

        expect(await copyText('nope')).toBe(false);
    });

    test('reports failure when writeText is not a function', async () => {
        setClipboard({ writeText: 'nope' });

        expect(await copyText('nope')).toBe(false);
    });

    test('reports failure when the write is rejected', async () => {
        writeText.mockRejectedValue(new Error('permission denied'));

        expect(await copyText('nope')).toBe(false);
    });
});

describe('copyPng', () => {
    const blob = new Blob([new Uint8Array([1, 2, 3])], { type: 'image/png' });

    test('writes a PNG clipboard item and reports success', async () => {
        expect(await copyPng(blob)).toBe(true);
        expect(write).toHaveBeenCalledTimes(1);
        const items = write.mock.calls[0]?.[0];
        expect(Array.isArray(items) && items.length).toBe(1);
        expect(pngOf(items?.[0])).toBe(blob);
    });

    test('reports failure when the clipboard API is missing', async () => {
        setClipboard(undefined);

        expect(await copyPng(blob)).toBe(false);
    });

    test('reports failure when ClipboardItem is unavailable', async () => {
        vi.stubGlobal('ClipboardItem', undefined);

        expect(await copyPng(blob)).toBe(false);
    });

    test('reports failure when the write is rejected', async () => {
        write.mockRejectedValue(new Error('insecure context'));

        expect(await copyPng(blob)).toBe(false);
    });
});
