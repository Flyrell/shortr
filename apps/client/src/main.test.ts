import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { mountApp } from './app';

vi.mock('./app', () => ({ mountApp: vi.fn() }));

const mountAppMock = vi.mocked(mountApp);

function setReadyState(state: string): void {
    Object.defineProperty(document, 'readyState', { value: state, configurable: true });
}

beforeEach(() => {
    vi.resetModules();
    mountAppMock.mockReset();
});

afterEach(() => {
    Reflect.deleteProperty(document, 'readyState');
});

describe('main', () => {
    test('mounts immediately when the document is already parsed', async () => {
        setReadyState('complete');

        await import('./main');

        expect(mountAppMock).toHaveBeenCalledWith(document);
    });

    test('waits for DOMContentLoaded while the document is still loading', async () => {
        setReadyState('loading');

        await import('./main');
        expect(mountAppMock).not.toHaveBeenCalled();

        document.dispatchEvent(new Event('DOMContentLoaded'));
        expect(mountAppMock).toHaveBeenCalledWith(document);
    });
});
