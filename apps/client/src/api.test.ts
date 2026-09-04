import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { postShorten } from './api';

const fetchMock = vi.fn<typeof fetch>();

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json', ...headers },
    });
}

beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
    vi.unstubAllGlobals();
});

describe('postShorten', () => {
    test('sends a JSON body to the shorten endpoint', async () => {
        fetchMock.mockResolvedValue(
            jsonResponse(201, { code: 'abc1234', shortUrl: 'https://s.io/abc1234', expiresAt: '2026-10-04T00:00:00Z' }),
        );

        await postShorten('https://example.com');

        const call = fetchMock.mock.calls[0];
        expect(call?.[0]).toBe('/api/shorten');
        expect(call?.[1]?.method).toBe('POST');
        expect(call?.[1]?.body).toBe('{"url":"https://example.com"}');
        expect(call?.[1]?.headers).toEqual({ 'Content-Type': 'application/json', Accept: 'application/json' });
    });

    test('parses a successful response', async () => {
        fetchMock.mockResolvedValue(
            jsonResponse(201, { code: 'abc1234', shortUrl: 'https://s.io/abc1234', expiresAt: '2026-10-04T00:00:00Z' }),
        );

        expect(await postShorten('https://example.com')).toEqual({ kind: 'ok', shortUrl: 'https://s.io/abc1234' });
    });

    test('turns a malformed success body into an error', async () => {
        fetchMock.mockResolvedValue(new Response('<html>nope</html>', { status: 201 }));

        expect(await postShorten('https://example.com')).toEqual({
            kind: 'error',
            status: 201,
            code: 'invalid_response',
            message: 'The server sent a response we could not understand.',
        });
    });

    test('turns a success body with missing fields into an error', async () => {
        fetchMock.mockResolvedValue(jsonResponse(201, { shortUrl: 'https://s.io/abc1234' }));

        expect(await postShorten('https://example.com')).toMatchObject({ kind: 'error', code: 'invalid_response' });
    });

    test.each([
        [400, 'invalid_url', 'The URL is not valid.'],
        [403, 'forbidden', 'Blocked.'],
        [413, 'body_too_large', 'Too big.'],
        [415, 'unsupported_media_type', 'Send JSON.'],
        [503, 'unavailable', 'Try later.'],
        [500, 'internal', 'Boom.'],
    ])('passes through the %i error code and message', async (status, code, message) => {
        fetchMock.mockResolvedValue(jsonResponse(status, { error: code, message }));

        expect(await postShorten('https://example.com')).toEqual({ kind: 'error', status, code, message });
    });

    test.each([
        [400, 'http_400', 'That does not look like a URL we can shorten.'],
        [403, 'http_403', 'This client is not allowed to shorten URLs.'],
        [413, 'http_413', 'That URL is too long.'],
        [415, 'http_415', 'The server rejected the request format.'],
        [429, 'http_429', 'Too many requests.'],
        [503, 'http_503', 'The service is temporarily unavailable.'],
        [418, 'http_418', 'The request failed with status 418.'],
    ])('falls back to a default code and message for %i', async (status, code, message) => {
        fetchMock.mockResolvedValue(new Response('not json at all', { status }));

        expect(await postShorten('https://example.com')).toEqual({ kind: 'error', status, code, message });
    });

    test('reads the retry delay from the Retry-After header', async () => {
        fetchMock.mockResolvedValue(
            jsonResponse(429, { error: 'rate_limited', message: 'Slow down.' }, { 'Retry-After': '42' }),
        );

        expect(await postShorten('https://example.com')).toEqual({
            kind: 'error',
            status: 429,
            code: 'rate_limited',
            message: 'Slow down.',
            retryAfterSeconds: 42,
        });
    });

    test('omits the retry delay when the header is absent', async () => {
        fetchMock.mockResolvedValue(jsonResponse(429, { error: 'rate_limited', message: 'Slow down.' }));

        expect(await postShorten('https://example.com')).not.toHaveProperty('retryAfterSeconds');
    });

    test('omits the retry delay when the header is not a number', async () => {
        fetchMock.mockResolvedValue(
            jsonResponse(429, { error: 'rate_limited', message: 'Slow down.' }, { 'Retry-After': 'soon' }),
        );

        expect(await postShorten('https://example.com')).not.toHaveProperty('retryAfterSeconds');
    });

    test('propagates network failures', async () => {
        fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));

        await expect(postShorten('https://example.com')).rejects.toThrow('Failed to fetch');
    });
});
