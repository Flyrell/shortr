import { toCanvas } from 'qrcode';
import { beforeEach, describe, expect, test, vi } from 'vitest';
import { renderQr } from './qr';

vi.mock('qrcode', () => ({ toCanvas: vi.fn() }));

const drawn = vi.mocked(toCanvas);

beforeEach(() => {
    drawn.mockReset();
    drawn.mockResolvedValue(undefined);
});

describe('renderQr', () => {
    test('draws the short URL on a canvas and returns a data URL and a PNG blob', async () => {
        const dataUrls = vi.spyOn(HTMLCanvasElement.prototype, 'toDataURL');

        const image = await renderQr('https://s.io/abc1234', document);

        const call = drawn.mock.calls[0];
        expect(call?.[0]).toBeInstanceOf(HTMLCanvasElement);
        expect(call?.[1]).toBe('https://s.io/abc1234');
        expect(call?.[2]).toEqual({ margin: 2, scale: 8, errorCorrectionLevel: 'M' });
        expect(dataUrls.mock.contexts[0]).toBe(call?.[0]);
        expect(dataUrls).toHaveBeenCalledWith('image/png');
        expect(image.dataUrl.startsWith('data:image/png;base64,')).toBe(true);
        expect(image.blob.type).toBe('image/png');

        dataUrls.mockRestore();
    });

    test('propagates a drawing failure', async () => {
        drawn.mockRejectedValue(new Error('input too long'));

        await expect(renderQr('https://s.io/abc1234', document)).rejects.toThrow('input too long');
    });

    test('rejects when the canvas produces no blob', async () => {
        const toBlob = vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation((callback) => {
            callback(null);
        });

        await expect(renderQr('https://s.io/abc1234', document)).rejects.toThrow('produced no PNG blob');

        toBlob.mockRestore();
    });
});
