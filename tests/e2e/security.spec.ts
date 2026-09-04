import { expect, test } from '@playwright/test';

test('serves the page with indexing and injection defences', async ({ request }) => {
    const response = await request.get('/');
    expect(response.status()).toBe(200);
    expect(response.headers()['x-robots-tag']).toContain('noindex');
    expect(response.headers()['content-security-policy']).toContain("default-src 'self'");
});

test('disallows every path in robots.txt', async ({ request }) => {
    const response = await request.get('/robots.txt');
    expect(response.status()).toBe(200);
    expect(await response.text()).toContain('Disallow: /');
});

test('blocks known crawlers', async ({ request }) => {
    const response = await request.get('/', { headers: { 'User-Agent': 'Googlebot' } });
    expect(response.status()).toBe(403);
});
