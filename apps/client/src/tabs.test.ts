import { beforeEach, describe, expect, test, vi } from 'vitest';
import { requireElement } from './dom';
import { createTabs } from './tabs';

const MARKUP = `
<div id="tabs" role="tablist">
    <button id="tab-shorten" type="button" role="tab" data-value="shorten" aria-controls="panel"
        aria-selected="true" tabindex="0">A</button>
    <button id="tab-qr" type="button" role="tab" data-value="qr" aria-controls="panel"
        aria-selected="false" tabindex="-1">B</button>
    <button id="tab-x" type="button" role="tab" data-value="x" aria-controls="panel"
        aria-selected="false" tabindex="-1">C</button>
</div>
<div id="panel" role="tabpanel" aria-labelledby="tab-shorten"></div>`;

function tablist(): HTMLElement {
    document.body.innerHTML = MARKUP;
    return requireElement(document, '#tabs', HTMLElement);
}

function press(id: string, key: string): KeyboardEvent {
    const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true });
    requireElement(document, id, HTMLButtonElement).dispatchEvent(event);
    return event;
}

function selection(): (string | null)[] {
    return ['#tab-shorten', '#tab-qr', '#tab-x'].map((id) =>
        requireElement(document, id, HTMLButtonElement).getAttribute('aria-selected'),
    );
}

function pill(): string {
    return requireElement(document, '#tabs', HTMLElement).style.getPropertyValue('--pill');
}

function panelLabel(): string | null {
    return requireElement(document, '#panel', HTMLElement).getAttribute('aria-labelledby');
}

const onSelect = vi.fn<(value: string) => void>();

beforeEach(() => {
    onSelect.mockReset();
});

describe('createTabs', () => {
    test('select applies the roving tabindex without calling back', () => {
        const tabs = createTabs(tablist(), onSelect);

        tabs.select('qr');

        expect(selection()).toEqual(['false', 'true', 'false']);
        expect(requireElement(document, '#tab-shorten', HTMLButtonElement).tabIndex).toBe(-1);
        expect(requireElement(document, '#tab-qr', HTMLButtonElement).tabIndex).toBe(0);
        expect(onSelect).not.toHaveBeenCalled();
    });

    test('select slides the pill onto the selected tab', () => {
        const tabs = createTabs(tablist(), onSelect);

        tabs.select('x');
        expect(pill()).toBe('200%');

        tabs.select('shorten');
        expect(pill()).toBe('0%');
    });

    test('select leaves the pill where it is for a value no tab carries', () => {
        const tabs = createTabs(tablist(), onSelect);
        tabs.select('qr');

        tabs.select('nope');

        expect(pill()).toBe('100%');
    });

    test('select names the controlled panel after the selected tab', () => {
        const tabs = createTabs(tablist(), onSelect);

        tabs.select('qr');

        expect(panelLabel()).toBe('tab-qr');
    });

    test('select ignores a value no tab carries', () => {
        const tabs = createTabs(tablist(), onSelect);

        tabs.select('nope');

        expect(selection()).toEqual(['true', 'false', 'false']);
        expect(requireElement(document, '#tab-shorten', HTMLButtonElement).tabIndex).toBe(0);
        expect(panelLabel()).toBe('tab-shorten');
    });

    test('a click selects the tab and reports the value', () => {
        createTabs(tablist(), onSelect);

        requireElement(document, '#tab-qr', HTMLButtonElement).click();

        expect(selection()).toEqual(['false', 'true', 'false']);
        expect(onSelect).toHaveBeenCalledWith('qr');
        expect(panelLabel()).toBe('tab-qr');
    });

    test('ArrowRight moves to the next tab and wraps around', () => {
        createTabs(tablist(), onSelect);

        press('#tab-shorten', 'ArrowRight');
        expect(onSelect).toHaveBeenLastCalledWith('qr');

        press('#tab-x', 'ArrowRight');
        expect(onSelect).toHaveBeenLastCalledWith('shorten');
        expect(document.activeElement).toBe(requireElement(document, '#tab-shorten', HTMLButtonElement));
    });

    test('ArrowLeft moves to the previous tab and wraps around', () => {
        createTabs(tablist(), onSelect);

        press('#tab-shorten', 'ArrowLeft');

        expect(onSelect).toHaveBeenLastCalledWith('x');
        expect(selection()).toEqual(['false', 'false', 'true']);
    });

    test('Home and End jump to the first and last tab', () => {
        createTabs(tablist(), onSelect);

        press('#tab-qr', 'End');
        expect(onSelect).toHaveBeenLastCalledWith('x');

        press('#tab-x', 'Home');
        expect(onSelect).toHaveBeenLastCalledWith('shorten');
        expect(selection()).toEqual(['true', 'false', 'false']);
    });

    test('Home and End on a boundary tab still suppress the browser default', () => {
        createTabs(tablist(), onSelect);

        expect(press('#tab-shorten', 'Home').defaultPrevented).toBe(true);
        expect(press('#tab-x', 'End').defaultPrevented).toBe(true);
        expect(onSelect).not.toHaveBeenCalled();
    });

    test('other keys are left to the browser', () => {
        createTabs(tablist(), onSelect);

        expect(press('#tab-shorten', 'a').defaultPrevented).toBe(false);
        expect(onSelect).not.toHaveBeenCalled();
    });

    test('ignores tabs without a value', () => {
        document.body.innerHTML = '<div id="tabs" role="tablist"><button type="button" role="tab">A</button></div>';
        createTabs(requireElement(document, '#tabs', HTMLElement), onSelect);

        requireElement(document, '#tabs button', HTMLButtonElement).click();

        expect(onSelect).not.toHaveBeenCalled();
    });
});
