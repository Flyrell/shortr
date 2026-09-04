export type Expiry = { readonly relative: string; readonly absolute: string };

type Unit = { readonly name: string; readonly ms: number };

const UNITS: readonly Unit[] = [
    { name: 'day', ms: 86_400_000 },
    { name: 'hour', ms: 3_600_000 },
    { name: 'minute', ms: 60_000 },
];

const SUB_MINUTE = 'in less than a minute';

// Pinned locale: the expiry copy is day-first in every locale, so the line never reorders.
const ABSOLUTE_LOCALE = 'en-GB';
const ABSOLUTE_FORMAT: Intl.DateTimeFormatOptions = { day: 'numeric', month: 'short', year: 'numeric' };

export function describeExpiry(expiresAt: string, now: Date): Expiry | null {
    const date = new Date(expiresAt);
    const time = date.getTime();
    if (Number.isNaN(time)) {
        return null;
    }
    const remaining = time - now.getTime();
    if (remaining <= 0) {
        return null;
    }
    return { relative: toRelative(remaining), absolute: date.toLocaleDateString(ABSOLUTE_LOCALE, ABSOLUTE_FORMAT) };
}

function toRelative(remainingMs: number): string {
    for (const unit of UNITS) {
        if (Math.floor(remainingMs / unit.ms) < 1) {
            continue;
        }
        const count = Math.round(remainingMs / unit.ms);
        return `in ${count} ${unit.name}${count === 1 ? '' : 's'}`;
    }
    return SUB_MINUTE;
}
