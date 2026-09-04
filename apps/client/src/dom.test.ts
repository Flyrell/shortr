import { beforeEach, describe, expect, test } from 'vitest';
import { requireElement } from './dom';

beforeEach(() => {
    document.body.innerHTML = '<div id="box"><button id="go" type="button">Go</button></div>';
});

describe('requireElement', () => {
    test('returns the element narrowed to the requested type', () => {
        const button = requireElement(document, '#go', HTMLButtonElement);

        expect(button.type).toBe('button');
    });

    test('throws when nothing matches the selector', () => {
        expect(() => requireElement(document, '#missing', HTMLElement)).toThrow(/no element matches #missing/);
    });

    test('throws when the element has another type', () => {
        expect(() => requireElement(document, '#box', HTMLButtonElement)).toThrow(/#box is not a/);
    });

    test('scopes the lookup to the given root', () => {
        const box = requireElement(document, '#box', HTMLDivElement);

        expect(requireElement(box, 'button', HTMLButtonElement).id).toBe('go');
    });
});
