import { create } from 'qrcode';

export type QrCode = { readonly size: number; readonly isDark: (row: number, column: number) => boolean };

const SVG_NAMESPACE = 'http://www.w3.org/2000/svg';
const PNG_TYPE = 'image/png';
const SVG_QUIET_MODULES = 3;
const PNG_QUIET_MODULES = 4;
const PNG_MODULE_PX = 10;
const DRAW_MS = 300;
const PAPER = '#ffffff';
const INK = '#12131a';

export function encodeQr(text: string): QrCode {
    const { modules } = create(text, { errorCorrectionLevel: 'M' });
    return {
        size: modules.size,
        isDark: (row, column) => modules.data[row * modules.size + column] === 1,
    };
}

export function drawQrSvg(doc: Document, qr: QrCode): SVGElement {
    const span = qr.size + SVG_QUIET_MODULES * 2;
    const svg = doc.createElementNS(SVG_NAMESPACE, 'svg');
    svg.setAttribute('viewBox', `0 0 ${span} ${span}`);
    svg.setAttribute('aria-hidden', 'true');
    const middle = (qr.size - 1) / 2;
    const furthest = Math.hypot(middle, middle);
    for (let row = 0; row < qr.size; row++) {
        for (let column = 0; column < qr.size; column++) {
            if (!qr.isDark(row, column)) {
                continue;
            }
            svg.append(rect(doc, row, column, delayMs(row - middle, column - middle, furthest)));
        }
    }
    return svg;
}

export function drawQrPng(doc: Document, qr: QrCode): Promise<Blob> {
    const span = (qr.size + PNG_QUIET_MODULES * 2) * PNG_MODULE_PX;
    const canvas = doc.createElement('canvas');
    canvas.width = span;
    canvas.height = span;
    const context = canvas.getContext('2d');
    if (context === null) {
        return Promise.reject(new Error('qr: the canvas offers no 2d context'));
    }
    context.fillStyle = PAPER;
    context.fillRect(0, 0, span, span);
    context.fillStyle = INK;
    for (let row = 0; row < qr.size; row++) {
        for (let column = 0; column < qr.size; column++) {
            if (qr.isDark(row, column)) {
                const offset = PNG_QUIET_MODULES * PNG_MODULE_PX;
                context.fillRect(
                    column * PNG_MODULE_PX + offset,
                    row * PNG_MODULE_PX + offset,
                    PNG_MODULE_PX,
                    PNG_MODULE_PX,
                );
            }
        }
    }
    return toPngBlob(canvas);
}

function rect(doc: Document, row: number, column: number, delay: number): SVGElement {
    const element = doc.createElementNS(SVG_NAMESPACE, 'rect');
    element.setAttribute('x', String(column + SVG_QUIET_MODULES));
    element.setAttribute('y', String(row + SVG_QUIET_MODULES));
    element.setAttribute('width', '1');
    element.setAttribute('height', '1');
    element.setAttribute('rx', '0.15');
    element.style.setProperty('--d', `${delay}ms`);
    return element;
}

function delayMs(rowOffset: number, columnOffset: number, furthest: number): number {
    if (furthest <= 0) {
        return 0;
    }
    return Math.round((Math.hypot(rowOffset, columnOffset) / furthest) * DRAW_MS);
}

function toPngBlob(canvas: HTMLCanvasElement): Promise<Blob> {
    return new Promise((resolve, reject) => {
        canvas.toBlob((blob) => {
            if (blob === null) {
                reject(new Error('qr: the canvas produced no PNG blob'));
                return;
            }
            resolve(blob);
        }, PNG_TYPE);
    });
}
