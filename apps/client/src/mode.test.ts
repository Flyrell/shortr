import { beforeEach, describe, expect, test, vi } from 'vitest';
import { isMode, load, save } from './mode';

const STORAGE_KEY = 'shortr.mode';

beforeEach(() => {
    window.localStorage.clear();
});

describe('isMode', () => {
    test.each([
        ['shorten', true],
        ['qr', true],
        ['QR', false],
        ['', false],
        [null, false],
        [undefined, false],
        [1, false],
    ])('narrows %j to %s', (value, expected) => {
        expect(isMode(value)).toBe(expected);
    });
});

describe('load', () => {
    test('defaults to shorten when nothing is stored', () => {
        expect(load()).toBe('shorten');
    });

    test('returns the stored mode', () => {
        window.localStorage.setItem(STORAGE_KEY, 'qr');

        expect(load()).toBe('qr');
    });

    test('falls back to shorten for an unknown stored value', () => {
        window.localStorage.setItem(STORAGE_KEY, 'rainbow');

        expect(load()).toBe('shorten');
    });

    test('falls back to shorten when storage is unavailable', () => {
        const getItem = vi.spyOn(window.localStorage, 'getItem').mockImplementation(() => {
            throw new Error('storage disabled');
        });

        expect(load()).toBe('shorten');

        getItem.mockRestore();
    });
});

describe('save', () => {
    test('writes the mode to storage', () => {
        save('qr');

        expect(window.localStorage.getItem(STORAGE_KEY)).toBe('qr');
    });

    test('swallows storage failures', () => {
        const setItem = vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
            throw new Error('quota exceeded');
        });

        expect(() => save('shorten')).not.toThrow();

        setItem.mockRestore();
    });
});
