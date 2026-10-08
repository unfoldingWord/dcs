import {addDelegatedEventListener, hideElem, showElem} from '../utils/dom.ts';

// Expand/collapse the ingredients, relations and healthcheck tables on the repo
// metadata page. Delegated listeners because inline onclick handlers are blocked
// by the CSP script nonce policy.
export function initDCSMetadataToggles() {
  addDelegatedEventListener<HTMLButtonElement, MouseEvent>(document, 'click', 'button.dcs-toggle-table', (button, e) => {
    e.preventDefault();
    const content = document.querySelector<HTMLElement>(button.getAttribute('aria-controls')!)!;
    const expanded = button.getAttribute('aria-expanded') === 'true';
    if (expanded) {
      hideElem(content);
      button.setAttribute('aria-expanded', 'false');
      button.textContent = button.getAttribute('data-expand-text')!;
    } else {
      showElem(content);
      button.setAttribute('aria-expanded', 'true');
      button.textContent = button.getAttribute('data-collapse-text')!;
    }
  });
}
