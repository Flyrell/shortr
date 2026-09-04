import type { Expiry } from './expiry';

export type Card = { readonly element: HTMLElement; readonly markCopied: (pulse: boolean) => void };

export type QrRow = {
    readonly shortUrl: string;
    readonly svg: SVGElement;
    readonly expiry: Expiry | null;
    readonly onCopyImage: () => void;
    readonly onCopyLink: () => void;
};

const CORNERS = ['tl', 'tr', 'bl', 'br'];

export function createCard(doc: Document, heading: string): Card {
    const element = doc.createElement('div');
    element.className = 'card';
    for (const corner of CORNERS) {
        element.append(span(doc, `tick ${corner}`, ''));
    }
    const pill = span(doc, 'pill', 'Copied');
    const head = doc.createElement('div');
    head.className = 'card-head';
    head.append(span(doc, 'micro', heading), pill);
    element.append(head);

    return {
        element,
        markCopied(pulse) {
            pill.classList.add('on');
            if (!pulse) {
                return;
            }
            element.classList.remove('pulse');
            // Reading the layout flushes the class removal, so the animation restarts on every copy.
            element.getBoundingClientRect();
            element.classList.add('pulse');
        },
    };
}

export function createLinkButton(doc: Document, shortUrl: string, onCopy: () => void): HTMLButtonElement {
    const button = copyButton(doc, 'link ring', shortUrl, onCopy);
    const { host, code } = splitLink(shortUrl);
    const codeLine = span(doc, 'link-code', '');
    for (const [index, character] of Array.from(code).entries()) {
        const glyph = span(doc, '', character);
        glyph.style.setProperty('--i', String(index));
        codeLine.append(glyph);
    }
    button.append(span(doc, 'link-host', host), codeLine);
    return button;
}

export function createExpiryLine(doc: Document, expiry: Expiry): HTMLElement {
    const line = doc.createElement('p');
    line.className = 'expiry';
    const relative = doc.createElement('b');
    relative.textContent = expiry.relative;
    line.append(
        doc.createTextNode('Expires '),
        relative,
        doc.createTextNode(' \u00B7 '),
        span(doc, '', expiry.absolute),
    );
    return line;
}

export function createQrRow(doc: Document, parts: QrRow): HTMLElement {
    const image = doc.createElement('button');
    image.type = 'button';
    image.id = 'copy-png';
    image.className = 'qr-btn ring';
    image.setAttribute('aria-label', 'Copy the QR code image to clipboard');
    image.append(parts.svg);
    image.addEventListener('click', parts.onCopyImage);

    const side = doc.createElement('div');
    side.className = 'qr-side';
    const link = copyButton(doc, 'link-2 ring', parts.shortUrl, parts.onCopyLink);
    link.textContent = parts.shortUrl;
    side.append(link);
    if (parts.expiry !== null) {
        side.append(createExpiryLine(doc, parts.expiry));
    }
    side.append(span(doc, 'hint', 'Encoded above. Both buttons copy.'));

    const row = doc.createElement('div');
    row.className = 'qr-row';
    row.append(image, side);
    return row;
}

function copyButton(doc: Document, className: string, shortUrl: string, onCopy: () => void): HTMLButtonElement {
    const button = doc.createElement('button');
    button.type = 'button';
    button.id = 'copy-link';
    button.className = className;
    button.setAttribute('aria-label', `Copy ${shortUrl} to clipboard`);
    button.addEventListener('click', onCopy);
    return button;
}

function span(doc: Document, className: string, text: string): HTMLSpanElement {
    const element = doc.createElement('span');
    if (className !== '') {
        element.className = className;
    }
    element.textContent = text;
    return element;
}

function splitLink(shortUrl: string): { host: string; code: string } {
    const withoutScheme = shortUrl.replace(/^https?:\/\//, '');
    const cut = withoutScheme.lastIndexOf('/');
    if (cut === -1) {
        return { host: '', code: withoutScheme };
    }
    return { host: withoutScheme.slice(0, cut + 1), code: withoutScheme.slice(cut + 1) };
}
