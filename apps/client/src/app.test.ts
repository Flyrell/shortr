import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { mountApp } from './app';
import { copyPng, copyText } from './clipboard';
import { requireElement } from './dom';
import { renderQr } from './qr';

vi.mock('./clipboard', () => ({ copyText: vi.fn(), copyPng: vi.fn() }));
vi.mock('./qr', () => ({ renderQr: vi.fn() }));

const copyTextMock = vi.mocked(copyText);
const copyPngMock = vi.mocked(copyPng);
const renderQrMock = vi.mocked(renderQr);
const fetchMock = vi.fn<typeof fetch>();

const bodyMarkup = readBodyMarkup();
const qrBlob = new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' });

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

function shortenedResponse(): Response {
    return jsonResponse(201, {
        code: 'abc1234',
        shortUrl: 'https://s.io/abc1234',
        expiresAt: '2026-10-04T00:00:00Z',
    });
}

function submit(url: string): void {
    requireElement(document, '#url', HTMLInputElement).value = url;
    requireElement(document, '#shorten-form', HTMLFormElement).dispatchEvent(
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

function resultText(): string {
    return requireElement(document, '#result', HTMLElement).textContent ?? '';
}

function toastText(): string {
    return requireElement(document, '#toast', HTMLElement).textContent ?? '';
}

beforeEach(() => {
    document.body.innerHTML = bodyMarkup;
    window.localStorage.clear();
    copyTextMock.mockReset().mockResolvedValue(true);
    copyPngMock.mockReset().mockResolvedValue(true);
    renderQrMock.mockReset().mockResolvedValue({ dataUrl: 'data:image/png;base64,AAAA', blob: qrBlob });
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
        expect(requireElement(document, '#panel', HTMLElement).getAttribute('aria-labelledby')).toBe('tab-qr');
        expect(requireElement(document, '#url-label', HTMLLabelElement).textContent).toBe('URL to encode');
        expect(requireElement(document, '#submit', HTMLButtonElement).textContent).toBe('Create QR code');
    });

    test('starts in the shorten mode when nothing is stored', () => {
        mountApp(document);

        expect(requireElement(document, '#tab-shorten', HTMLButtonElement).getAttribute('aria-selected')).toBe('true');
        expect(requireElement(document, '#panel', HTMLElement).getAttribute('aria-labelledby')).toBe('tab-shorten');
        expect(requireElement(document, '#url-label', HTMLLabelElement).textContent).toBe('URL to shorten');
        expect(requireElement(document, '#submit', HTMLButtonElement).textContent).toBe('Shorten');
    });

    test('persists the mode when another tab is picked and clears the result', async () => {
        mountApp(document);
        submit('https://example.com/long');
        await vi.waitFor(() => expect(resultText()).toContain('https://s.io/abc1234'));

        requireElement(document, '#tab-qr', HTMLButtonElement).click();

        expect(window.localStorage.getItem('shortr.mode')).toBe('qr');
        expect(resultText()).toBe('');
        expect(requireElement(document, '#url-label', HTMLLabelElement).textContent).toBe('URL to encode');
    });

    test('announces the result from a polite live region', () => {
        mountApp(document);

        const region = requireElement(document, '#result', HTMLElement);
        expect(region.getAttribute('role')).toBe('region');
        expect(region.getAttribute('aria-live')).toBe('polite');
    });

    test('shows an inline alert when the input is empty', () => {
        mountApp(document);

        submit('   ');

        const alert = requireElement(document, '#result p', HTMLParagraphElement);
        expect(alert.getAttribute('role')).toBe('alert');
        expect(alert.textContent).toBe('Enter a URL first.');
        expect(fetchMock).not.toHaveBeenCalled();
    });

    test('renders the short URL, auto-copies it and toasts', async () => {
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(copyTextMock).toHaveBeenCalledWith('https://s.io/abc1234'));
        const button = requireElement(document, '#result button', HTMLButtonElement);
        expect(button.textContent).toBe('https://s.io/abc1234');
        expect(button.getAttribute('aria-label')).toBe('Copy short URL https://s.io/abc1234');
        expect(toastText()).toBe('Copied to clipboard');
    });

    test('copies again and toasts when the result button is activated', async () => {
        mountApp(document);
        submit('https://example.com/long');
        await vi.waitFor(() => expect(toastText()).toBe('Copied to clipboard'));
        requireElement(document, '#toast', HTMLElement).textContent = '';

        requireElement(document, '#result button', HTMLButtonElement).click();

        await vi.waitFor(() => expect(copyTextMock).toHaveBeenCalledTimes(2));
        expect(toastText()).toBe('Copied to clipboard');
    });

    test('toasts the manual fallback when copying fails', async () => {
        copyTextMock.mockResolvedValue(false);
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(toastText()).toBe('Copy failed, click the result to copy manually'));
    });

    test('renders a QR image, auto-copies the PNG and offers the short URL', async () => {
        window.localStorage.setItem('shortr.mode', 'qr');
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(copyPngMock).toHaveBeenCalledWith(qrBlob));
        expect(renderQrMock).toHaveBeenCalledWith('https://s.io/abc1234', document);
        const image = requireElement(document, '#result img', HTMLImageElement);
        expect(image.alt).toBe('QR code for https://s.io/abc1234');
        expect(
            requireElement(document, '#result .result__button--qr', HTMLButtonElement).getAttribute('aria-label'),
        ).toBe('Copy QR code for https://s.io/abc1234');
        expect(image.getAttribute('src')).toBe('data:image/png;base64,AAAA');
        expect(document.querySelectorAll('#result button')).toHaveLength(2);
        expect(toastText()).toBe('Copied to clipboard');
    });

    test('copies the PNG again when the QR result is activated', async () => {
        window.localStorage.setItem('shortr.mode', 'qr');
        mountApp(document);
        submit('https://example.com/long');
        await vi.waitFor(() => expect(copyPngMock).toHaveBeenCalledTimes(1));

        requireElement(document, '#result .result__button--qr', HTMLButtonElement).click();

        await vi.waitFor(() => expect(copyPngMock).toHaveBeenCalledTimes(2));
    });

    test('copies the short URL from the secondary button in QR mode', async () => {
        window.localStorage.setItem('shortr.mode', 'qr');
        mountApp(document);
        submit('https://example.com/long');
        await vi.waitFor(() => expect(copyPngMock).toHaveBeenCalledTimes(1));

        requireElement(document, '#result button:not(.result__button--qr)', HTMLButtonElement).click();

        await vi.waitFor(() => expect(copyTextMock).toHaveBeenCalledWith('https://s.io/abc1234'));
        expect(toastText()).toBe('Copied to clipboard');
    });

    test('reports a QR rendering failure inline', async () => {
        window.localStorage.setItem('shortr.mode', 'qr');
        renderQrMock.mockRejectedValue(new Error('too much data'));
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(resultText()).toBe('The QR code could not be rendered.'));
        expect(copyPngMock).not.toHaveBeenCalled();
    });

    test('shows the retry delay for a rate limited request', async () => {
        fetchMock.mockResolvedValue(
            jsonResponse(429, { error: 'rate_limited', message: 'Too many requests.' }, { 'Retry-After': '30' }),
        );
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(resultText()).toBe('Too many requests. Try again in 30 seconds.'));
        expect(requireElement(document, '#result p', HTMLParagraphElement).getAttribute('role')).toBe('alert');
    });

    test('reports a network failure inline', async () => {
        fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));
        mountApp(document);

        submit('https://example.com/long');

        await vi.waitFor(() =>
            expect(resultText()).toBe('The server could not be reached. Check your connection and try again.'),
        );
    });

    test('renders the result for the mode the request was submitted in', async () => {
        const release = deferFetch();
        mountApp(document);

        submit('https://example.com/long');
        requireElement(document, '#tab-qr', HTMLButtonElement).click();
        release();

        await vi.waitFor(() => expect(copyTextMock).toHaveBeenCalledWith('https://s.io/abc1234'));
        expect(renderQrMock).not.toHaveBeenCalled();
        expect(document.querySelector('#result img')).toBeNull();
    });

    test('re-enables the form as soon as the result is on screen', async () => {
        copyTextMock.mockReturnValue(new Promise<boolean>(() => undefined));
        mountApp(document);
        const button = requireElement(document, '#submit', HTMLButtonElement);

        submit('https://example.com/long');

        await vi.waitFor(() => expect(resultText()).toContain('https://s.io/abc1234'));
        expect(button.disabled).toBe(false);
    });

    test('disables the submit button while the request is in flight', async () => {
        const release = deferFetch();
        mountApp(document);
        const button = requireElement(document, '#submit', HTMLButtonElement);

        submit('https://example.com/long');
        await vi.waitFor(() => expect(button.disabled).toBe(true));

        submit('https://example.com/another');
        expect(fetchMock).toHaveBeenCalledTimes(1);

        release();
        await vi.waitFor(() => expect(button.disabled).toBe(false));
    });
});
