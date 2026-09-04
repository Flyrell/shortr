import type { BuildOptions } from 'esbuild';
import { beforeEach, describe, expect, test, vi } from 'vitest';
import { run } from './build';

type WatchListener = (event: string, filename: string | null) => void;

const mocks = vi.hoisted(() => ({
    build: vi.fn<(options: unknown) => Promise<void>>(),
    context: vi.fn<(options: unknown) => Promise<{ watch: () => Promise<void> }>>(),
    watch: vi.fn<(directory: string, listener: (event: string, filename: string | null) => void) => void>(),
    copyFile: vi.fn<(from: string, to: string) => Promise<void>>(),
    mkdir: vi.fn<(directory: string, options: unknown) => Promise<void>>(),
    ctxWatch: vi.fn<() => Promise<void>>(),
}));

vi.mock('esbuild', () => ({ build: mocks.build, context: mocks.context }));
vi.mock('node:fs', () => ({ default: { watch: mocks.watch }, watch: mocks.watch }));
vi.mock('node:fs/promises', () => ({
    default: { copyFile: mocks.copyFile, mkdir: mocks.mkdir },
    copyFile: mocks.copyFile,
    mkdir: mocks.mkdir,
}));

function copiedNames(): string[] {
    return mocks.copyFile.mock.calls.map((call) => call[1].split('/').slice(-1).join(''));
}

function watchListener(): WatchListener {
    const listener = mocks.watch.mock.calls[0]?.[1];
    if (listener === undefined) {
        throw new Error('the source directory was never watched');
    }
    return listener;
}

function isBuildOptions(value: unknown): value is BuildOptions {
    return typeof value === 'object' && value !== null && 'entryPoints' in value && Array.isArray(value.entryPoints);
}

function buildOptions(value: unknown): BuildOptions {
    if (!isBuildOptions(value)) {
        throw new Error('esbuild was never given a build configuration');
    }
    return value;
}

beforeEach(() => {
    mocks.build.mockReset().mockResolvedValue(undefined);
    mocks.ctxWatch.mockReset().mockResolvedValue(undefined);
    mocks.context.mockReset().mockResolvedValue({ watch: mocks.ctxWatch });
    mocks.watch.mockReset();
    mocks.copyFile.mockReset().mockResolvedValue(undefined);
    mocks.mkdir.mockReset().mockResolvedValue(undefined);
});

describe('run', () => {
    test('builds once when --watch is absent', async () => {
        await run([]);

        expect(mocks.build).toHaveBeenCalledTimes(1);
        expect(mocks.context).not.toHaveBeenCalled();
        expect(mocks.watch).not.toHaveBeenCalled();
    });

    test('bundles the two entry points into dist/assets without source maps', async () => {
        await run([]);

        const options = buildOptions(mocks.build.mock.calls[0]?.[0]);
        expect(options.entryPoints).toEqual([
            expect.stringMatching(/src\/main\.ts$/),
            expect.stringMatching(/src\/styles\.css$/),
        ]);
        expect(options.outdir).toMatch(/dist$/);
        expect(options.entryNames).toBe('assets/[name]');
        expect(options.bundle).toBe(true);
        expect(options.minify).toBe(true);
        expect(options.sourcemap).toBe(false);
    });

    test('emits font files under assets and references them from the site root', async () => {
        await run([]);

        const options = buildOptions(mocks.build.mock.calls[0]?.[0]);
        expect(options.assetNames).toBe('assets/[name]-[hash]');
        expect(options.publicPath).toBe('/');
        expect(options.loader).toEqual({ '.woff2': 'file', '.woff': 'file' });
    });

    test('creates dist and copies every static file', async () => {
        await run([]);

        expect(mocks.mkdir).toHaveBeenCalledWith(expect.stringMatching(/dist$/), { recursive: true });
        expect(copiedNames()).toEqual(['index.html', 'robots.txt', 'favicon.svg']);
    });

    test('starts the esbuild and static file watchers with --watch', async () => {
        await run(['node', 'scripts/build.ts', '--watch']);

        expect(mocks.build).not.toHaveBeenCalled();
        expect(mocks.context).toHaveBeenCalledTimes(1);
        expect(mocks.ctxWatch).toHaveBeenCalledTimes(1);
        expect(mocks.watch).toHaveBeenCalledWith(expect.stringMatching(/src$/), expect.any(Function));
        expect(buildOptions(mocks.context.mock.calls[0]?.[0]).sourcemap).toBe(true);
    });

    test('re-copies only when a static file changes', async () => {
        await run(['--watch']);
        mocks.copyFile.mockClear();

        watchListener()('change', 'main.ts');
        watchListener()('change', null);
        expect(mocks.copyFile).not.toHaveBeenCalled();

        watchListener()('change', 'index.html');
        await vi.waitFor(() => expect(copiedNames()).toEqual(['index.html', 'robots.txt', 'favicon.svg']));
    });

    test('logs and survives a copy that fails after a rename', async () => {
        await run(['--watch']);
        const logged = vi.spyOn(console, 'error').mockImplementation(() => undefined);
        mocks.copyFile.mockRejectedValue(new Error('ENOENT'));

        watchListener()('rename', 'index.html');
        await vi.waitFor(() => expect(logged).toHaveBeenCalledTimes(1));

        logged.mockRestore();
    });
});
