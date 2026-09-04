type ElementKind<T extends Element> = abstract new (...args: never[]) => T;

export function requireElement<T extends Element>(root: ParentNode, selector: string, kind: ElementKind<T>): T {
    const element = root.querySelector(selector);
    if (element === null) {
        throw new Error(`dom: no element matches ${selector}`);
    }
    if (!(element instanceof kind)) {
        throw new Error(`dom: element ${selector} is not a ${kind.name}`);
    }
    return element;
}
