import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { createToast } from './toast';

const VISIBLE_MS = 2600;

function element(): HTMLElement {
    const paragraph = document.createElement('p');
    paragraph.className = 'toast';
    paragraph.setAttribute('role', 'status');
    paragraph.setAttribute('aria-live', 'polite');
    document.body.replaceChildren(paragraph);
    return paragraph;
}

function recordWrites(target: HTMLElement): string[] {
    const writes: string[] = [];
    Object.defineProperty(target, 'textContent', {
        configurable: true,
        get: (): string => writes[writes.length - 1] ?? '',
        set: (value: string): void => {
            writes.push(value);
        },
    });
    return writes;
}

beforeEach(() => {
    vi.useFakeTimers();
});

afterEach(() => {
    vi.useRealTimers();
});

describe('createToast', () => {
    test('shows the message inside the live region', () => {
        const target = element();

        createToast(target).show('Copied to clipboard');

        expect(target.textContent).toBe('Copied to clipboard');
        expect(target.classList.contains('on')).toBe(true);
        expect(target.getAttribute('aria-live')).toBe('polite');
    });

    test('hides the message once it has been visible long enough', () => {
        const target = element();

        createToast(target).show('Copied to clipboard');
        vi.advanceTimersByTime(VISIBLE_MS - 1);
        expect(target.classList.contains('on')).toBe(true);

        vi.advanceTimersByTime(1);
        expect(target.classList.contains('on')).toBe(false);
        expect(target.textContent).toBe('');
    });

    test('empties the region before repeating an identical message', () => {
        const target = element();
        const writes = recordWrites(target);
        const toast = createToast(target);

        toast.show('Copied to clipboard');
        toast.show('Copied to clipboard');

        expect(writes).toEqual(['', 'Copied to clipboard', '', 'Copied to clipboard']);
    });

    test('restarts the timer when a new message arrives', () => {
        const target = element();
        const toast = createToast(target);

        toast.show('first');
        vi.advanceTimersByTime(VISIBLE_MS - 100);
        toast.show('second');
        vi.advanceTimersByTime(VISIBLE_MS - 100);

        expect(target.textContent).toBe('second');
        expect(target.classList.contains('on')).toBe(true);

        vi.advanceTimersByTime(100);
        expect(target.textContent).toBe('');
    });
});
