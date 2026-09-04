import { describe, expect, test, vi } from 'vitest';
import { requireElement } from './dom';
import type { Expiry } from './expiry';
import { createCard, createExpiryLine, createLinkButton, createQrRow } from './result';

const SHORT_URL = 'https://s.io/abc123DEF456';
const EXPIRY: Expiry = { relative: 'in 30 days', absolute: '4 Oct 2026' };

function svg(): SVGElement {
    return document.createElementNS('http://www.w3.org/2000/svg', 'svg');
}

describe('createCard', () => {
    test('frames the card with four corner ticks and a heading', () => {
        const card = createCard(document, 'short link');

        expect(card.element.className).toBe('card');
        expect(Array.from(card.element.querySelectorAll('.tick')).map((tick) => tick.className)).toEqual([
            'tick tl',
            'tick tr',
            'tick bl',
            'tick br',
        ]);
        expect(requireElement(card.element, '.micro', HTMLElement).textContent).toBe('short link');
        expect(requireElement(card.element, '.pill', HTMLElement).textContent).toBe('Copied');
    });

    test('reveals the pill without pulsing on the first copy', () => {
        const card = createCard(document, 'short link');

        card.markCopied(false);

        expect(requireElement(card.element, '.pill', HTMLElement).classList.contains('on')).toBe(true);
        expect(card.element.classList.contains('pulse')).toBe(false);
    });

    test('pulses the card when a copy is repeated', () => {
        const card = createCard(document, 'short link');

        card.markCopied(true);
        card.markCopied(true);

        expect(card.element.classList.contains('pulse')).toBe(true);
    });
});

describe('createLinkButton', () => {
    test('splits the short URL into a host line and one span per code character', () => {
        const button = createLinkButton(document, SHORT_URL, () => undefined);

        expect(requireElement(button, '.link-host', HTMLElement).textContent).toBe('s.io/');
        const glyphs = Array.from(button.querySelectorAll('.link-code span'));
        expect(glyphs).toHaveLength(12);
        expect(glyphs.map((glyph) => glyph.textContent).join('')).toBe('abc123DEF456');
    });

    test('staggers the code characters with an index custom property', () => {
        const button = createLinkButton(document, SHORT_URL, () => undefined);

        const glyphs = Array.from(button.querySelectorAll<HTMLElement>('.link-code span'));
        expect(glyphs.map((glyph) => glyph.style.getPropertyValue('--i')).slice(0, 3)).toEqual(['0', '1', '2']);
    });

    test('labels and reports a copy request', () => {
        const onCopy = vi.fn();
        const button = createLinkButton(document, SHORT_URL, onCopy);

        button.click();

        expect(button.id).toBe('copy-link');
        expect(button.getAttribute('aria-label')).toBe(`Copy ${SHORT_URL} to clipboard`);
        expect(onCopy).toHaveBeenCalledTimes(1);
    });
});

describe('createExpiryLine', () => {
    test('highlights the relative part and separates it from the date', () => {
        const line = createExpiryLine(document, EXPIRY);

        expect(line.className).toBe('expiry');
        expect(requireElement(line, 'b', HTMLElement).textContent).toBe('in 30 days');
        expect(requireElement(line, 'span', HTMLElement).textContent).toBe('4 Oct 2026');
        expect(line.textContent).toBe('Expires in 30 days \u00B7 4 Oct 2026');
    });
});

describe('createQrRow', () => {
    test('places the image button, the short link, the expiry and the hint', () => {
        const onCopyImage = vi.fn();
        const onCopyLink = vi.fn();

        const row = createQrRow(document, {
            shortUrl: SHORT_URL,
            svg: svg(),
            expiry: EXPIRY,
            onCopyImage,
            onCopyLink,
        });

        const image = requireElement(row, '#copy-png', HTMLButtonElement);
        expect(image.getAttribute('aria-label')).toBe('Copy the QR code image to clipboard');
        expect(image.querySelector('svg')).not.toBeNull();
        const link = requireElement(row, '#copy-link', HTMLButtonElement);
        expect(link.textContent).toBe(SHORT_URL);
        expect(requireElement(row, '.expiry b', HTMLElement).textContent).toBe('in 30 days');
        expect(requireElement(row, '.hint', HTMLElement).textContent).toBe('Encoded above. Both buttons copy.');

        image.click();
        link.click();
        expect(onCopyImage).toHaveBeenCalledTimes(1);
        expect(onCopyLink).toHaveBeenCalledTimes(1);
    });

    test('leaves the expiry out when it is unknown', () => {
        const row = createQrRow(document, {
            shortUrl: SHORT_URL,
            svg: svg(),
            expiry: null,
            onCopyImage: () => undefined,
            onCopyLink: () => undefined,
        });

        expect(row.querySelector('.expiry')).toBeNull();
    });
});
