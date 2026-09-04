import { watch } from 'node:fs';
import { copyFile, mkdir } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { BuildOptions } from 'esbuild';
import { build, context } from 'esbuild';

const projectDir = dirname(dirname(fileURLToPath(import.meta.url)));
const srcDir = join(projectDir, 'src');
const distDir = join(projectDir, 'dist');
const staticFiles = ['index.html', 'robots.txt', 'favicon.svg'];

function buildOptions(watchMode: boolean): BuildOptions {
    return {
        entryPoints: [join(srcDir, 'main.ts'), join(srcDir, 'styles.css')],
        outdir: distDir,
        entryNames: 'assets/[name]',
        assetNames: 'assets/[name]-[hash]',
        // The served origin is the site root, so font url() references resolve to /assets/... .
        publicPath: '/',
        loader: { '.woff2': 'file', '.woff': 'file' },
        bundle: true,
        minify: true,
        sourcemap: watchMode,
        format: 'esm',
        target: 'es2022',
        logLevel: 'info',
    };
}

function isStaticFile(filename: string | null): boolean {
    return filename !== null && staticFiles.includes(filename);
}

async function copyStaticFiles(): Promise<void> {
    await mkdir(distDir, { recursive: true });
    await Promise.all(staticFiles.map((name) => copyFile(join(srcDir, name), join(distDir, name))));
}

export async function run(argv: readonly string[]): Promise<void> {
    await copyStaticFiles();
    if (!argv.includes('--watch')) {
        await build(buildOptions(false));
        return;
    }
    const ctx = await context(buildOptions(true));
    await ctx.watch();
    watch(srcDir, (_event, filename) => {
        if (!isStaticFile(filename)) {
            return;
        }
        // A rename event can fire after the file is gone; a rejection here would kill the watcher.
        copyStaticFiles().catch((error: unknown) => {
            console.error(error);
        });
    });
}

if (import.meta.main) {
    await run(process.argv);
}
