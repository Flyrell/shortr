import type { ShortenError, ShortenResult } from './api';
import { postShorten } from './api';
import { copyPng, copyText } from './clipboard';
import { requireElement } from './dom';
import type { Mode } from './mode';
import { isMode, load, save } from './mode';
import type { QrImage } from './qr';
import { renderQr } from './qr';
import { createTabs } from './tabs';
import { createToast } from './toast';

type AutoCopy = () => void;

const COPIED_MESSAGE = 'Copied to clipboard';
const COPY_FAILED_MESSAGE = 'Copy failed, click the result to copy manually';

const FORM_TEXT: Record<Mode, { label: string; submit: string }> = {
    shorten: { label: 'URL to shorten', submit: 'Shorten' },
    qr: { label: 'URL to encode', submit: 'Create QR code' },
};

export function mountApp(root: Document): void {
    const tablist = requireElement(root, '#tabs', HTMLElement);
    const form = requireElement(root, '#shorten-form', HTMLFormElement);
    const label = requireElement(root, '#url-label', HTMLLabelElement);
    const input = requireElement(root, '#url', HTMLInputElement);
    const submit = requireElement(root, '#submit', HTMLButtonElement);
    const result = requireElement(root, '#result', HTMLElement);
    const toast = createToast(requireElement(root, '#toast', HTMLElement));

    let mode: Mode = 'shorten';
    let pending = false;

    function applyMode(value: Mode): void {
        mode = value;
        label.textContent = FORM_TEXT[value].label;
        submit.textContent = FORM_TEXT[value].submit;
    }

    const tabs = createTabs(tablist, (value) => {
        if (!isMode(value)) {
            return;
        }
        applyMode(value);
        save(value);
        result.replaceChildren();
    });
    applyMode(load());
    tabs.select(mode);

    function resultButton(ariaLabel: string): HTMLButtonElement {
        const button = root.createElement('button');
        button.type = 'button';
        button.className = 'result__button';
        button.setAttribute('aria-label', ariaLabel);
        return button;
    }

    async function copyTextAndToast(text: string): Promise<void> {
        toast.show((await copyText(text)) ? COPIED_MESSAGE : COPY_FAILED_MESSAGE);
    }

    async function copyPngAndToast(blob: Blob): Promise<void> {
        toast.show((await copyPng(blob)) ? COPIED_MESSAGE : COPY_FAILED_MESSAGE);
    }

    function linkButton(shortUrl: string): HTMLButtonElement {
        const button = resultButton(`Copy short URL ${shortUrl}`);
        button.textContent = shortUrl;
        button.addEventListener('click', () => {
            void copyTextAndToast(shortUrl);
        });
        return button;
    }

    function qrButton(shortUrl: string, image: QrImage): HTMLButtonElement {
        const button = resultButton(`Copy QR code for ${shortUrl}`);
        button.classList.add('result__button--qr');
        const picture = root.createElement('img');
        picture.src = image.dataUrl;
        picture.alt = `QR code for ${shortUrl}`;
        button.append(picture);
        button.addEventListener('click', () => {
            void copyPngAndToast(image.blob);
        });
        return button;
    }

    function renderError(message: string): void {
        const paragraph = root.createElement('p');
        paragraph.className = 'result__error';
        paragraph.setAttribute('role', 'alert');
        paragraph.textContent = message;
        result.replaceChildren(paragraph);
    }

    async function renderResult(shortUrl: string, requestMode: Mode): Promise<AutoCopy | null> {
        if (requestMode === 'shorten') {
            result.replaceChildren(linkButton(shortUrl));
            return () => {
                void copyTextAndToast(shortUrl);
            };
        }
        let image: QrImage;
        try {
            image = await renderQr(shortUrl, root);
        } catch {
            renderError('The QR code could not be rendered.');
            return null;
        }
        result.replaceChildren(qrButton(shortUrl, image), linkButton(shortUrl));
        return () => {
            void copyPngAndToast(image.blob);
        };
    }

    async function handleSubmit(): Promise<void> {
        if (pending) {
            return;
        }
        const url = input.value.trim();
        if (url === '') {
            renderError('Enter a URL first.');
            return;
        }
        const requestMode = mode;
        pending = true;
        submit.disabled = true;
        let autoCopy: AutoCopy | null = null;
        try {
            const response = await requestShortUrl(url);
            if (response.kind === 'error') {
                renderError(describeError(response));
                return;
            }
            autoCopy = await renderResult(response.shortUrl, requestMode);
        } finally {
            pending = false;
            submit.disabled = false;
        }
        // Deliberately not awaited: a clipboard permission prompt must never keep the form disabled.
        autoCopy?.();
    }

    form.addEventListener('submit', (event) => {
        event.preventDefault();
        void handleSubmit();
    });
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
