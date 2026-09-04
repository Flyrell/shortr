import { mountApp } from './app';

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        mountApp(document);
    });
} else {
    mountApp(document);
}
