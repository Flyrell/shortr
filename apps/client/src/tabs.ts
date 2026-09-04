type TabsController = { select: (value: string) => void };

export function createTabs(tablist: HTMLElement, onSelect: (value: string) => void): TabsController {
    const tabs = collectTabs(tablist);

    function render(value: string): void {
        const selected = tabs.find((tab) => tab.dataset.value === value);
        if (selected === undefined) {
            return;
        }
        for (const tab of tabs) {
            tab.setAttribute('aria-selected', String(tab === selected));
            tab.tabIndex = tab === selected ? 0 : -1;
        }
        labelPanel(selected);
    }

    function activate(tab: HTMLButtonElement): void {
        const value = tab.dataset.value;
        if (value === undefined) {
            return;
        }
        render(value);
        onSelect(value);
    }

    tabs.forEach((tab, index) => {
        tab.addEventListener('click', () => {
            activate(tab);
        });
        tab.addEventListener('keydown', (event) => {
            const next = nextIndex(event.key, index, tabs.length);
            if (next === null) {
                return;
            }
            event.preventDefault();
            const target = tabs[next];
            if (target === undefined || target === tab) {
                return;
            }
            target.focus();
            activate(target);
        });
    });

    return { select: render };
}

function collectTabs(tablist: HTMLElement): HTMLButtonElement[] {
    const tabs: HTMLButtonElement[] = [];
    for (const candidate of tablist.querySelectorAll('[role="tab"]')) {
        if (candidate instanceof HTMLButtonElement) {
            tabs.push(candidate);
        }
    }
    return tabs;
}

function labelPanel(tab: HTMLButtonElement): void {
    const panelId = tab.getAttribute('aria-controls');
    if (panelId === null || tab.id === '') {
        return;
    }
    const panel = tab.ownerDocument.getElementById(panelId);
    if (panel === null) {
        return;
    }
    panel.setAttribute('aria-labelledby', tab.id);
}

function nextIndex(key: string, index: number, count: number): number | null {
    switch (key) {
        case 'ArrowLeft':
            return (index - 1 + count) % count;
        case 'ArrowRight':
            return (index + 1) % count;
        case 'Home':
            return 0;
        case 'End':
            return count - 1;
        default:
            return null;
    }
}
