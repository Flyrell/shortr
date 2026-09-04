import type { ShortenError, ShortenResult } from './api';
import { postShorten } from './api';
import { requireElement } from './dom';
import { describeExpiry } from './expiry';
import type { Mode } from './mode';
import { isMode, load, save } from './mode';
import type { AutoCopy } from './output';
import { createOutput } from './output';
import { createTabs } from './tabs';
import { createToast } from './toast';

type ModeText = { readonly label: string; readonly submit: string; readonly pending: string };

const MODE_TEXT: Record<Mode, ModeText> = {
    shorten: { label: 'URL to shorten', submit: 'Shorten', pending: 'Shortening' },
    qr: { label: 'URL to encode', submit: 'Create QR code', pending: 'Encoding' },
};

export function mountApp(root: Document): void {
    const tablist = requireElement(root, '#tabs', HTMLElement);
    const form = requireElement(root, '#panel', HTMLFormElement);
    const label = requireElement(root, '#url-label', HTMLLabelElement);
    const input = requireElement(root, '#url', HTMLInputElement);
    const submit = requireElement(root, '#submit', HTMLButtonElement);
    const submitLabel = requireElement(root, '#submit-label', HTMLElement);
    const output = createOutput({
        doc: root,
        result: requireElement(root, '#result', HTMLElement),
        errorBox: requireElement(root, '#error', HTMLElement),
        toast: createToast(requireElement(root, '#toast', HTMLElement)),
    });

    let mode: Mode = 'shorten';
    let pending = false;

    function applyMode(value: Mode): void {
        mode = value;
        label.textContent = MODE_TEXT[value].label;
        submitLabel.textContent = MODE_TEXT[value].submit;
    }

    const tabs = createTabs(tablist, (value) => {
        if (!isMode(value)) {
            return;
        }
        applyMode(value);
        save(value);
        output.clear();
    });
    applyMode(load());
    tabs.select(mode);

    function setPending(value: boolean): void {
        pending = value;
        submit.setAttribute('aria-busy', String(value));
        submitLabel.textContent = value ? MODE_TEXT[mode].pending : MODE_TEXT[mode].submit;
        submit.querySelector('.dots')?.remove();
        if (!value) {
            submit.removeAttribute('aria-disabled');
            return;
        }
        submit.setAttribute('aria-disabled', 'true');
        submit.append(createDots(root));
    }

    async function handleSubmit(): Promise<void> {
        if (pending) {
            return;
        }
        const url = input.value.trim();
        if (url === '') {
            output.showError('check', 'Enter a URL first.');
            input.focus();
            return;
        }
        const requestMode = mode;
        output.clear();
        setPending(true);
        let autoCopy: AutoCopy | null = null;
        try {
            const response = await requestShortUrl(url);
            if (response.kind === 'error') {
                output.showError(response.code, describeError(response));
                return;
            }
            const expiry = describeExpiry(response.expiresAt, new Date());
            autoCopy =
                requestMode === 'shorten'
                    ? output.renderLink(response.shortUrl, expiry)
                    : await output.renderQr(response.shortUrl, expiry);
        } finally {
            setPending(false);
        }
        // Deliberately not awaited: a clipboard permission prompt must never keep the form pending.
        autoCopy?.();
    }

    form.addEventListener('submit', (event) => {
        event.preventDefault();
        void handleSubmit();
    });
}

function createDots(root: Document): HTMLElement {
    const dots = root.createElement('span');
    dots.className = 'dots';
    dots.setAttribute('aria-hidden', 'true');
    dots.append(root.createElement('i'), root.createElement('i'), root.createElement('i'));
    return dots;
}

async function requestShortUrl(url: string): Promise<ShortenResult> {
    try {
        return await postShorten(url);
    } catch {
        return {
            kind: 'error',
            status: 0,
            code: 'network_error',
            message: 'The server could not be reached. Check your connection and try again.',
        };
    }
}

function describeError(error: ShortenError): string {
    if (error.status !== 429 || error.retryAfterSeconds === undefined) {
        return error.message;
    }
    const seconds = error.retryAfterSeconds;
    return `${error.message} Try again in ${seconds} ${seconds === 1 ? 'second' : 'seconds'}.`;
}
