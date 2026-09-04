export type Mode = 'shorten' | 'qr';

const STORAGE_KEY = 'shortr.mode';
const DEFAULT_MODE: Mode = 'shorten';

export function isMode(value: unknown): value is Mode {
    return value === 'shorten' || value === 'qr';
}

export function load(): Mode {
    try {
        const stored: unknown = window.localStorage.getItem(STORAGE_KEY);
        return isMode(stored) ? stored : DEFAULT_MODE;
    } catch {
        return DEFAULT_MODE;
    }
}

export function save(mode: Mode): void {
    try {
        window.localStorage.setItem(STORAGE_KEY, mode);
    } catch {
        return;
    }
}
