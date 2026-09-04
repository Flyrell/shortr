import { describe, expect, test } from 'vitest';
import { describeExpiry } from './expiry';

const NOW = new Date('2026-09-04T12:00:00Z');

function relative(expiresAt: string): string | undefined {
    return describeExpiry(expiresAt, NOW)?.relative;
}

describe('describeExpiry', () => {
    test.each([
        ['2026-10-04T12:00:00Z', 'in 30 days'],
        ['2026-10-04T11:59:55Z', 'in 30 days'],
        ['2026-09-05T21:36:00Z', 'in 1 day'],
        ['2026-09-05T12:00:00Z', 'in 1 day'],
        ['2026-09-05T11:54:00Z', 'in 24 hours'],
        ['2026-09-04T13:00:00Z', 'in 1 hour'],
        ['2026-09-04T12:40:00Z', 'in 40 minutes'],
        ['2026-09-04T12:01:30Z', 'in 2 minutes'],
        ['2026-09-04T12:01:00Z', 'in 1 minute'],
    ])('reports %s as %s', (expiresAt, expected) => {
        expect(relative(expiresAt)).toBe(expected);
    });

    test('formats the absolute date day first, regardless of the host locale', () => {
        const expiry = describeExpiry('2026-10-04T12:00:00Z', NOW);

        expect(expiry?.absolute).toMatch(/^\d{1,2} Oct 2026$/);
    });

    test('measures the distance from the injected clock', () => {
        expect(describeExpiry('2026-10-04T12:00:00Z', new Date('2026-10-02T12:00:00Z'))?.relative).toBe('in 2 days');
    });

    test.each([['not-a-date'], [''], ['2026-13-45T00:00:00Z']])('returns nothing for %j', (expiresAt) => {
        expect(describeExpiry(expiresAt, NOW)).toBeNull();
    });

    test('keeps the line when less than a minute is left', () => {
        const expiry = describeExpiry('2026-09-04T12:00:30Z', NOW);

        expect(expiry?.relative).toBe('in less than a minute');
        expect(expiry?.absolute).toMatch(/ 2026$/);
    });

    test('returns nothing when the link has already expired', () => {
        expect(describeExpiry('2026-09-04T11:00:00Z', NOW)).toBeNull();
    });
});
