import { toCanvas } from 'qrcode';

export type QrImage = { dataUrl: string; blob: Blob };

const PNG_TYPE = 'image/png';

export async function renderQr(shortUrl: string, doc: Document): Promise<QrImage> {
    const canvas = doc.createElement('canvas');
    await toCanvas(canvas, shortUrl, { margin: 2, scale: 8, errorCorrectionLevel: 'M' });
    return { dataUrl: canvas.toDataURL(PNG_TYPE), blob: await toPngBlob(canvas) };
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
