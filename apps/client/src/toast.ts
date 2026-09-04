export type ToastController = { readonly show: (message: string) => void };

const VISIBLE_CLASS = 'on';
const VISIBLE_MS = 2600;

export function createToast(element: HTMLElement): ToastController {
    let timer = 0;
    return {
        show(message: string): void {
            window.clearTimeout(timer);
            // Emptied first so repeating the same message is still a change the live region announces.
            element.textContent = '';
            element.textContent = message;
            element.classList.add(VISIBLE_CLASS);
            timer = window.setTimeout(() => {
                element.classList.remove(VISIBLE_CLASS);
                element.textContent = '';
            }, VISIBLE_MS);
        },
    };
}
