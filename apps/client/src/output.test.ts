import { beforeEach, describe, expect, test, vi } from 'vitest';
import { copyPng, copyText } from './clipboard';
import { requireElement } from './dom';
import type { Expiry } from './expiry';
import type { Output } from './output';
import { createOutput } from './output';
import { drawQrPng, drawQrSvg, encodeQr } from './qr';

vi.mock('./clipboard', () => ({ copyText: vi.fn(), copyPng: vi.fn() }));
vi.mock('./qr', () => ({ encodeQr: vi.fn(), drawQrSvg: vi.fn(), drawQrPng: vi.fn() }));

const copyTextMock = vi.mocked(copyText);
const copyPngMock = vi.mocked(copyPng);
const encodeQrMock = vi.mocked(encodeQr);
const drawQrSvgMock = vi.mocked(drawQrSvg);
const drawQrPngMock = vi.mocked(drawQrPng);

const SHORT_URL = 'https://s.io/abc123DEF456';
const EXPIRY: Expiry = { relative: 'in 30 days', absolute: '4 Oct 2026' };
const PNG = new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' });

const messages: string[] = [];

function mount(): Output {
    document.body.innerHTML = '<p id="error" hidden></p><div id="result"></div>';
    return createOutput({
        doc: document,
        result: requireElement(document, '#result', HTMLElement),
        errorBox: requireElement(document, '#error', HTMLElement),
        toast: {
            show(message: string): void {
                messages.push(message);
            },
        },
    });
}

function result(): HTMLElement {
    return requireElement(document, '#result', HTMLElement);
}

function errorBox(): HTMLElement {
    return requireElement(document, '#error', HTMLElement);
}

beforeEach(() => {
    messages.length = 0;
    copyTextMock.mockReset().mockResolvedValue(true);
    copyPngMock.mockReset().mockResolvedValue(true);
    encodeQrMock.mockReset().mockReturnValue({ size: 1, isDark: () => true });
    drawQrSvgMock.mockReset().mockReturnValue(document.createElementNS('http://www.w3.org/2000/svg', 'svg'));
    drawQrPngMock.mockReset().mockResolvedValue(PNG);
});

describe('createOutput', () => {
    test('showError writes a status marker beside the message and reveals the alert', () => {
        const output = mount();

        output.showError('429', 'Too many requests.');

        expect(requireElement(document, '#error b', HTMLElement).textContent).toBe('429');
        expect(requireElement(document, '#error span', HTMLElement).textContent).toBe('Too many requests.');
        expect(errorBox().hidden).toBe(false);
        expect(result().childElementCount).toBe(0);
    });

    test('clear empties the result and hides the alert again', () => {
        const output = mount();
        output.showError('429', 'Too many requests.');

        output.clear();

        expect(errorBox().hidden).toBe(true);
        expect(errorBox().textContent).toBe('');
        expect(result().childElementCount).toBe(0);
    });

    test('renderLink shows the code, the expiry line and copies on demand', async () => {
        const output = mount();

        const autoCopy = output.renderLink(SHORT_URL, EXPIRY);
        autoCopy();

        expect(result().querySelectorAll('.link-code span')).toHaveLength(12);
        expect(requireElement(document, '#result .expiry b', HTMLElement).textContent).toBe('in 30 days');
        await vi.waitFor(() => expect(messages).toEqual(['Copied to clipboard']));
        expect(copyTextMock).toHaveBeenCalledWith(SHORT_URL);
        expect(requireElement(document, '#result .pill', HTMLElement).classList.contains('on')).toBe(true);
        expect(requireElement(document, '#result .card', HTMLElement).classList.contains('pulse')).toBe(false);
    });

    test('renderLink pulses the card when the result is copied again', async () => {
        const output = mount();
        output.renderLink(SHORT_URL, EXPIRY);

        requireElement(document, '#copy-link', HTMLButtonElement).click();

        await vi.waitFor(() =>
            expect(requireElement(document, '#result .card', HTMLElement).classList.contains('pulse')).toBe(true),
        );
    });

    test('renderLink omits the expiry line when the expiry is unknown', () => {
        const output = mount();

        output.renderLink(SHORT_URL, null);

        expect(result().querySelector('.expiry')).toBeNull();
    });

    test('renderLink reports a copy failure', async () => {
        copyTextMock.mockResolvedValue(false);
        const output = mount();

        output.renderLink(SHORT_URL, EXPIRY)();

        await vi.waitFor(() => expect(messages).toEqual(['Copy failed, click the result to copy manually']));
    });

    test('renderQr draws the code, keeps the expiry line and copies the PNG', async () => {
        const output = mount();

        const autoCopy = await output.renderQr(SHORT_URL, EXPIRY);
        autoCopy?.();

        expect(encodeQrMock).toHaveBeenCalledWith(SHORT_URL);
        expect(document.querySelector('#copy-png svg')).not.toBeNull();
        expect(requireElement(document, '#result .expiry b', HTMLElement).textContent).toBe('in 30 days');
        await vi.waitFor(() => expect(copyPngMock).toHaveBeenCalledWith(PNG));
        expect(messages).toEqual(['Copied to clipboard']);
    });

    test('renderQr reports an encoding failure instead of a card', async () => {
        encodeQrMock.mockImplementation(() => {
            throw new Error('too much data');
        });
        const output = mount();

        expect(await output.renderQr(SHORT_URL, EXPIRY)).toBeNull();

        expect(requireElement(document, '#error span', HTMLElement).textContent).toBe(
            'The QR code could not be rendered.',
        );
        expect(result().childElementCount).toBe(0);
    });
});
