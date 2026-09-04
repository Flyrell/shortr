type TextWriter = { writeText: (text: string) => Promise<void> };
type ItemWriter = { write: (items: ClipboardItem[]) => Promise<void> };

export async function copyText(text: string): Promise<boolean> {
    const clipboard = readClipboard();
    if (!hasWriteText(clipboard)) {
        return false;
    }
    try {
        await clipboard.writeText(text);
        return true;
    } catch {
        return false;
    }
}

export async function copyPng(blob: Blob): Promise<boolean> {
    const clipboard = readClipboard();
    if (!hasWrite(clipboard)) {
        return false;
    }
    const createItem = globalThis.ClipboardItem;
    if (typeof createItem !== 'function') {
        return false;
    }
    try {
        await clipboard.write([new createItem({ 'image/png': blob })]);
        return true;
    } catch {
        return false;
    }
}

function readClipboard(): unknown {
    const navigator: unknown = globalThis.navigator;
    if (!isRecord(navigator)) {
        return null;
    }
    return navigator.clipboard;
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null;
}

function hasWriteText(value: unknown): value is TextWriter {
    return isRecord(value) && typeof value.writeText === 'function';
}

function hasWrite(value: unknown): value is ItemWriter {
    return isRecord(value) && typeof value.write === 'function';
}
