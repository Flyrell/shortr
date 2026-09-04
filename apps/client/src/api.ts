export type ShortenError = {
    readonly kind: 'error';
    readonly status: number;
    readonly code: string;
    readonly message: string;
    readonly retryAfterSeconds?: number;
};

export type ShortenOk = { readonly kind: 'ok'; readonly shortUrl: string; readonly expiresAt: string };

export type ShortenResult = ShortenOk | ShortenError;

const SHORTEN_PATH = '/api/shorten';

export async function postShorten(url: string): Promise<ShortenResult> {
    const response = await fetch(SHORTEN_PATH, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ url }),
    });
    const body = await readJson(response);
    if (!response.ok) {
        return toError(response, body);
    }
    if (!isShortenBody(body)) {
        return {
            kind: 'error',
            status: response.status,
            code: 'invalid_response',
            message: 'The server sent a response we could not understand.',
        };
    }
    return { kind: 'ok', shortUrl: body.shortUrl, expiresAt: body.expiresAt };
}

async function readJson(response: Response): Promise<unknown> {
    try {
        const parsed: unknown = await response.json();
        return parsed;
    } catch {
        return null;
    }
}

function toError(response: Response, body: unknown): ShortenError {
    const failure: ShortenError = {
        kind: 'error',
        status: response.status,
        code: readCode(body, response.status),
        message: readMessage(body, response.status),
    };
    const retryAfterSeconds = readRetryAfterSeconds(response);
    if (retryAfterSeconds === null) {
        return failure;
    }
    return { ...failure, retryAfterSeconds };
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null;
}

function isShortenBody(value: unknown): value is { shortUrl: string; expiresAt: string } {
    return isRecord(value) && typeof value.shortUrl === 'string' && typeof value.expiresAt === 'string';
}

function readCode(body: unknown, status: number): string {
    if (isRecord(body) && typeof body.error === 'string' && body.error !== '') {
        return body.error;
    }
    return `http_${status}`;
}

function readMessage(body: unknown, status: number): string {
    if (isRecord(body) && typeof body.message === 'string' && body.message !== '') {
        return body.message;
    }
    return defaultMessage(status);
}

function defaultMessage(status: number): string {
    switch (status) {
        case 400:
            return 'That does not look like a URL we can shorten.';
        case 403:
            return 'This client is not allowed to shorten URLs.';
        case 413:
            return 'That URL is too long.';
        case 415:
            return 'The server rejected the request format.';
        case 429:
            return 'Too many requests.';
        case 503:
            return 'The service is temporarily unavailable.';
        default:
            return `The request failed with status ${status}.`;
    }
}

function readRetryAfterSeconds(response: Response): number | null {
    const header = response.headers.get('Retry-After');
    if (header === null) {
        return null;
    }
    const parsed = Number.parseInt(header, 10);
    return Number.isInteger(parsed) && parsed >= 0 ? parsed : null;
}
