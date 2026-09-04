import { copyPng, copyText } from './clipboard';
import type { Expiry } from './expiry';
import { drawQrPng, drawQrSvg, encodeQr } from './qr';
import type { Card } from './result';
import { createCard, createExpiryLine, createLinkButton, createQrRow } from './result';
import type { ToastController } from './toast';

export type AutoCopy = () => void;

export type Output = {
    readonly clear: () => void;
    readonly showError: (status: string, message: string) => void;
    readonly renderLink: (shortUrl: string, expiry: Expiry | null) => AutoCopy;
    readonly renderQr: (shortUrl: string, expiry: Expiry | null) => Promise<AutoCopy | null>;
};

export type OutputTargets = {
    readonly doc: Document;
    readonly result: HTMLElement;
    readonly errorBox: HTMLElement;
    readonly toast: ToastController;
};

const COPIED_MESSAGE = 'Copied to clipboard';
const COPY_FAILED_MESSAGE = 'Copy failed, click the result to copy manually';
const LINK_HEADING = 'short link';
const QR_HEADING = 'qr code';
const QR_FAILED_MESSAGE = 'The QR code could not be rendered.';

export function createOutput(targets: OutputTargets): Output {
    const { doc, result, errorBox, toast } = targets;

    function clear(): void {
        result.replaceChildren();
        errorBox.replaceChildren();
        errorBox.hidden = true;
    }

    function showError(status: string, message: string): void {
        result.replaceChildren();
        const marker = doc.createElement('b');
        marker.textContent = status;
        const text = doc.createElement('span');
        text.textContent = message;
        errorBox.replaceChildren(marker, text);
        errorBox.hidden = false;
    }

    async function copyTextAndToast(text: string, card: Card, pulse: boolean): Promise<void> {
        if (await copyText(text)) {
            card.markCopied(pulse);
            toast.show(COPIED_MESSAGE);
            return;
        }
        toast.show(COPY_FAILED_MESSAGE);
    }

    async function copyPngAndToast(blob: Blob, card: Card): Promise<void> {
        if (await copyPng(blob)) {
            card.markCopied(true);
            toast.show(COPIED_MESSAGE);
            return;
        }
        toast.show(COPY_FAILED_MESSAGE);
    }

    function appendExpiry(card: Card, expiry: Expiry | null): void {
        if (expiry !== null) {
            card.element.append(createExpiryLine(doc, expiry));
        }
    }

    function renderLink(shortUrl: string, expiry: Expiry | null): AutoCopy {
        const card = createCard(doc, LINK_HEADING);
        card.element.append(
            createLinkButton(doc, shortUrl, () => {
                void copyTextAndToast(shortUrl, card, true);
            }),
        );
        appendExpiry(card, expiry);
        result.replaceChildren(card.element);
        return () => {
            void copyTextAndToast(shortUrl, card, false);
        };
    }

    async function renderQr(shortUrl: string, expiry: Expiry | null): Promise<AutoCopy | null> {
        const card = createCard(doc, QR_HEADING);
        let svg: SVGElement;
        let blob: Blob;
        try {
            const encoded = encodeQr(shortUrl);
            svg = drawQrSvg(doc, encoded);
            blob = await drawQrPng(doc, encoded);
        } catch {
            showError('qr', QR_FAILED_MESSAGE);
            return null;
        }
        card.element.append(
            createQrRow(doc, {
                shortUrl,
                svg,
                expiry,
                onCopyImage: () => {
                    void copyPngAndToast(blob, card);
                },
                onCopyLink: () => {
                    void copyTextAndToast(shortUrl, card, true);
                },
            }),
        );
        result.replaceChildren(card.element);
        return () => {
            void copyPngAndToast(blob, card);
        };
    }

    return { clear, showError, renderLink, renderQr };
}
