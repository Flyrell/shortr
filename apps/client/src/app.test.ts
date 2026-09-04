import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { mountApp } from './app';
import { copyPng, copyText } from './clipboard';
import { requireElement } from './dom';
import { drawQrPng, drawQrSvg, encodeQr } from './qr';

vi.mock('./clipboard', () => ({ copyText: vi.fn(), copyPng: vi.fn() }));
vi.mock('./qr', () => ({ encodeQr: vi.fn(), drawQrSvg: vi.fn(), drawQrPng: vi.fn() }));

const copyTextMock = vi.mocked(copyText);
const copyPngMock = vi.mocked(copyPng);
const encodeQrMock = vi.mocked(encodeQr);
const drawQrSvgMock = vi.mocked(drawQrSvg);
const drawQrPngMock = vi.mocked(drawQrPng);
const fetchMock = vi.fn<typeof fetch>();

const bodyMarkup = readBodyMarkup();
const qrBlob = new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' });
const SHORT_URL = 'https://s.io/abc123DEF456';
const EXPIRES_AT = new Date(Date.now() + 30 * 86_400_000 + 3_600_000).toISOString();

function readBodyMarkup(): string {
    const html = readFileSync(join(import.meta.dirname, 'index.html'), 'utf8');
    const inner = /<body[^>]*>([\s\S]*)<\/body>/.exec(html)?.[1];
    if (inner === undefined) {
        throw new Error('index.html has no body element');
    }
    return inner;
}

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json', ...headers },
    });
}

function shortenedResponse(expiresAt = EXPIRES_AT): Response {
    return jsonResponse(201, { code: 'abc123DEF456', shortUrl: SHORT_URL, expiresAt });
}

function submit(url: string): void {
    requireElement(document, '#url', HTMLInputElement).value = url;
    requireElement(document, '#panel', HTMLFormElement).dispatchEvent(
        new Event('submit', { bubbles: true, cancelable: true }),
    );
}

function deferFetch(): () => void {
    let release = (): void => undefined;
    fetchMock.mockReturnValue(
        new Promise<Response>((resolve) => {
            release = () => {
                resolve(shortenedResponse());
            };
        }),
    );
    return () => {
        release();
    };
}

function codeText(): string {
    return requireElement(document, '#result .link-code', HTMLElement).textContent ?? '';
}

function alertText(): string {
    return requireElement(document, '#error span', HTMLElement).textContent ?? '';
}

function toastText(): string {
    return requireElement(document, '#toast', HTMLElement).textContent ?? '';
}

function submitButton(): HTMLButtonElement {
    return requireElement(document, '#submit', HTMLButtonElement);
}

beforeEach(() => {
    document.body.innerHTML = bodyMarkup;
    window.localStorage.clear();
    copyTextMock.mockReset().mockResolvedValue(true);
    copyPngMock.mockReset().mockResolvedValue(true);
    encodeQrMock.mockReset().mockReturnValue({ size: 1, isDark: () => true });
    drawQrSvgMock.mockReset().mockReturnValue(document.createElementNS('http://www.w3.org/2000/svg', 'svg'));
    drawQrPngMock.mockReset().mockResolvedValue(qrBlob);
    fetchMock.mockReset().mockResolvedValue(shortenedResponse());
    vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
    vi.unstubAllGlobals();
});

describe('mountApp', () => {
    test('starts in the mode stored in localStorage and labels the form for it', () => {
        window.localStorage.setItem('shortr.mode', 'qr');

        mountApp(document);

        expect(requireElement(document, '#tab-qr', HTMLButtonElement).getAttribute('aria-selected')).toBe('true');
        expect(requireElement(document, '#tab-shorten', HTMLButtonElement).tabIndex).toBe(-1);
        expect(requireElement(document, '#panel', HTMLFormElement).getAttribute('aria-labelledby')).toBe('tab-qr');
        expect(requireElement(document, '#url-label', HTMLLabelElement).textContent).toBe('URL to encode');
        expect(requireElement(document, '#submit-label', HTMLElement).textContent).toBe('Create QR code');
    });

    test('starts in the shorten mode when nothing is stored and parks the pill left', () => {
        mountApp(document);

        expect(requireElement(document, '#tab-shorten', HTMLButtonElement).getAttribute('aria-selected')).toBe('true');
        expect(requireElement(document, '#tabs', HTMLElement).style.getPropertyValue('--pill')).toBe('0%');
        expect(requireElement(document, '#url-label', HTMLLabelElement).textContent).toBe('URL to shorten');
        expect(requireElement(document, '#submit-label', HTMLElement).textContent).toBe('Shorten');
    });

    test('persists the mode when another tab is picked and clears the result', async () => {
        mountApp(document);
        submit('https://example.com/long');
        await vi.waitFor(() => expect(codeText()).toBe('abc123DEF456'));

        requireElement(document, '#tab-qr', HTMLButtonElement).click();

        expect(window.localStorage.getItem('shortr.mode')).toBe('qr');
        expect(requireElement(document, '#result', HTMLElement).childElementCount).toBe(0);
        expect(requireElement(document, '#tabs', HTMLElement).style.getPropertyValue('--pill')).toBe('100%');
    });

    test('announces the result from a polite live region', () => {
        mountApp(document);

        expect(requireElement(document, '#result', HTMLElement).getAttribute('aria-live')).toBe('polite');
        expect(requireElement(document, '#error', HTMLElement).getAttribute('role')).toBe('alert');
    });

    test('shows an inline alert when the input is empty', () => {
        mountApp(document);

        submit('   ');

        expect(requireElement(document, '#error b', HTMLElement).textContent).toBe('check');
        expect(alertText()).toBe('Enter a URL first.');
        expect(fetchMock).not.toHaveBeenCalled();
    });

    test('returns focus to the input when it is empty', () => {
        mountApp(document);
        requireElement(document, '#submit', HTMLButtonElement).focus();

        submit('   ');

        expect(document.activeElement).toBe(requireElement(document, '#url', HTMLInputElement));
    });

    test('renders the short URL with its expiry, auto-copies it and toasts', async () => {
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(copyTextMock).toHaveBeenCalledWith(SHORT_URL));
        expect(document.querySelectorAll('#result .link-code span')).toHaveLength(12);
        expect(requireElement(document, '#result .link-host', HTMLElement).textContent).toBe('s.io/');
        expect(requireElement(document, '#result .expiry b', HTMLElement).textContent).toBe('in 30 days');
        expect(requireElement(document, '#copy-link', HTMLButtonElement).getAttribute('aria-label')).toBe(
            `Copy ${SHORT_URL} to clipboard`,
        );
        expect(toastText()).toBe('Copied to clipboard');
    });

    test('leaves the expiry line out when the server sends an unusable date', async () => {
        fetchMock.mockResolvedValue(shortenedResponse('not-a-date'));
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(codeText()).toBe('abc123DEF456'));
        expect(document.querySelector('#result .expiry')).toBeNull();
    });

    test('copies again and toasts when the result button is activated', async () => {
        mountApp(document);
        submit('https://example.com/long');
        await vi.waitFor(() => expect(toastText()).toBe('Copied to clipboard'));
        requireElement(document, '#toast', HTMLElement).textContent = '';

        requireElement(document, '#copy-link', HTMLButtonElement).click();

        await vi.waitFor(() => expect(copyTextMock).toHaveBeenCalledTimes(2));
        expect(toastText()).toBe('Copied to clipboard');
    });

    test('toasts the manual fallback when copying fails', async () => {
        copyTextMock.mockResolvedValue(false);
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(toastText()).toBe('Copy failed, click the result to copy manually'));
    });

    test('renders a QR code, auto-copies the PNG and offers the short URL', async () => {
        window.localStorage.setItem('shortr.mode', 'qr');
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(copyPngMock).toHaveBeenCalledWith(qrBlob));
        expect(encodeQrMock).toHaveBeenCalledWith(SHORT_URL);
        expect(document.querySelector('#copy-png svg')).not.toBeNull();
        expect(requireElement(document, '#copy-link', HTMLButtonElement).textContent).toBe(SHORT_URL);
        expect(requireElement(document, '#result .expiry b', HTMLElement).textContent).toBe('in 30 days');
        expect(toastText()).toBe('Copied to clipboard');
    });

    test('copies the PNG again when the QR result is activated', async () => {
        window.localStorage.setItem('shortr.mode', 'qr');
        mountApp(document);
        submit('https://example.com/long');
        await vi.waitFor(() => expect(copyPngMock).toHaveBeenCalledTimes(1));

        requireElement(document, '#copy-png', HTMLButtonElement).click();

        await vi.waitFor(() => expect(copyPngMock).toHaveBeenCalledTimes(2));
    });

    test('copies the short URL from the secondary button in QR mode', async () => {
        window.localStorage.setItem('shortr.mode', 'qr');
        mountApp(document);
        submit('https://example.com/long');
        await vi.waitFor(() => expect(copyPngMock).toHaveBeenCalledTimes(1));

        requireElement(document, '#copy-link', HTMLButtonElement).click();

        await vi.waitFor(() => expect(copyTextMock).toHaveBeenCalledWith(SHORT_URL));
        expect(toastText()).toBe('Copied to clipboard');
    });

    test('reports a QR encoding failure inline', async () => {
        window.localStorage.setItem('shortr.mode', 'qr');
        encodeQrMock.mockImplementation(() => {
            throw new Error('too much data');
        });
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(alertText()).toBe('The QR code could not be rendered.'));
        expect(copyPngMock).not.toHaveBeenCalled();
    });

    test('shows the retry delay for a rate limited request', async () => {
        fetchMock.mockResolvedValue(
            jsonResponse(429, { error: 'rate_limited', message: 'Too many requests.' }, { 'Retry-After': '30' }),
        );
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(alertText()).toBe('Too many requests. Try again in 30 seconds.'));
        expect(requireElement(document, '#error b', HTMLElement).textContent).toBe('rate_limited');
    });

    test('reports a network failure inline', async () => {
        fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() =>
            expect(alertText()).toBe('The server could not be reached. Check your connection and try again.'),
        );
    });

    test('renders the result for the mode the request was submitted in', async () => {
        const release = deferFetch();
        mountApp(document);

        submit('https://example.com/long');
        requireElement(document, '#tab-qr', HTMLButtonElement).click();
        release();

        await vi.waitFor(() => expect(copyTextMock).toHaveBeenCalledWith(SHORT_URL));
        expect(encodeQrMock).not.toHaveBeenCalled();
        expect(document.querySelector('#copy-png')).toBeNull();
    });

    test('shows pending dots while the request is in flight and refuses a second submit', async () => {
        const release = deferFetch();
        mountApp(document);

        submit('https://example.com/long');
        await vi.waitFor(() => expect(submitButton().getAttribute('aria-disabled')).toBe('true'));
        expect(requireElement(document, '#submit-label', HTMLElement).textContent).toBe('Shortening');
        expect(document.querySelectorAll('#submit .dots i')).toHaveLength(3);

        submit('https://example.com/another');
        expect(fetchMock).toHaveBeenCalledTimes(1);

        release();
        await vi.waitFor(() => expect(submitButton().hasAttribute('aria-disabled')).toBe(false));
        expect(requireElement(document, '#submit-label', HTMLElement).textContent).toBe('Shorten');
        expect(document.querySelector('#submit .dots')).toBeNull();
    });

    test('stops pending as soon as the result is on screen', async () => {
        copyTextMock.mockReturnValue(new Promise<boolean>(() => undefined));
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(codeText()).toBe('abc123DEF456'));
        expect(submitButton().getAttribute('aria-busy')).toBe('false');
    });
});
