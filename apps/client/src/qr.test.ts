import { create } from 'qrcode';
import { afterEach, describe, expect, test } from 'vitest';
import { drawQrPng, drawQrSvg, encodeQr } from './qr';

const TEXT = 'https://s.io/abc123def456';

type Fill = { x: number; y: number; width: number; height: number };

function replaceOnCanvas(name: string, value: unknown): void {
    Object.defineProperty(HTMLCanvasElement.prototype, name, { configurable: true, writable: true, value });
}

function stubContext(fills: Fill[]): void {
    replaceOnCanvas('getContext', () => ({
        fillStyle: '',
        fillRect: (x: number, y: number, width: number, height: number) => {
            fills.push({ x, y, width, height });
        },
    }));
}

function stubBlob(blob: Blob | null): void {
    replaceOnCanvas('toBlob', (callback: (value: Blob | null) => void) => {
        callback(blob);
    });
}

const originalGetContext = HTMLCanvasElement.prototype.getContext;
const originalToBlob = HTMLCanvasElement.prototype.toBlob;

function darkModuleCount(): number {
    const { modules } = create(TEXT, { errorCorrectionLevel: 'M' });
    return modules.data.reduce((total, bit) => total + (bit === 1 ? 1 : 0), 0);
}

function delays(svg: SVGElement): number[] {
    return Array.from(svg.querySelectorAll('rect')).map((rect) =>
        Number.parseInt(rect.style.getPropertyValue('--d'), 10),
    );
}

afterEach(() => {
    replaceOnCanvas('getContext', originalGetContext);
    replaceOnCanvas('toBlob', originalToBlob);
});

describe('encodeQr', () => {
    test('exposes the module matrix of the encoded text', () => {
        const qr = encodeQr(TEXT);

        expect(qr.size).toBe(25);
        expect(qr.isDark(0, 0)).toBe(true);
        expect(qr.isDark(7, 7)).toBe(false);
    });

    test('rejects text that cannot be encoded', () => {
        expect(() => encodeQr('x'.repeat(5000))).toThrow();
    });
});

describe('drawQrSvg', () => {
    test('draws one rect per dark module inside a quiet zone', () => {
        const svg = drawQrSvg(document, encodeQr(TEXT));

        expect(svg.getAttribute('viewBox')).toBe('0 0 31 31');
        expect(svg.getAttribute('aria-hidden')).toBe('true');
        expect(svg.querySelectorAll('rect')).toHaveLength(darkModuleCount());
    });

    test('offsets every module by the quiet zone', () => {
        const svg = drawQrSvg(document, encodeQr(TEXT));

        const first = svg.querySelector('rect');
        expect(first?.getAttribute('x')).toBe('3');
        expect(first?.getAttribute('y')).toBe('3');
        expect(first?.getAttribute('width')).toBe('1');
    });

    test('delays each module by its distance from the centre', () => {
        const svg = drawQrSvg(document, encodeQr(TEXT));

        const found = delays(svg);
        expect(Math.min(...found)).toBeLessThan(60);
        expect(Math.max(...found)).toBe(300);
    });
});

describe('drawQrPng', () => {
    test('paints every dark module on a canvas with a four module quiet zone', async () => {
        const fills: Fill[] = [];
        const blob = new Blob([new Uint8Array([137, 80, 78, 71])], { type: 'image/png' });
        stubContext(fills);
        stubBlob(blob);

        await expect(drawQrPng(document, encodeQr(TEXT))).resolves.toBe(blob);

        expect(fills).toHaveLength(darkModuleCount() + 1);
        expect(fills[0]).toEqual({ x: 0, y: 0, width: 330, height: 330 });
        expect(fills[1]).toEqual({ x: 40, y: 40, width: 10, height: 10 });
    });

    test('rejects when the canvas offers no 2d context', async () => {
        await expect(drawQrPng(document, encodeQr(TEXT))).rejects.toThrow('no 2d context');
    });

    test('rejects when the canvas produces no blob', async () => {
        stubContext([]);
        stubBlob(null);

        await expect(drawQrPng(document, encodeQr(TEXT))).rejects.toThrow('produced no PNG blob');
    });
});
