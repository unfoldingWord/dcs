import {addDelegatedEventListener} from '../utils/dom.ts';

// Expand/collapse the ingredients, relations and healthcheck tables on the repo
// metadata page. Delegated listeners because inline onclick handlers are blocked
// by the CSP script nonce policy.
export function initDCSMetadataToggles() {
  addDelegatedEventListener<HTMLAnchorElement, MouseEvent>(document, 'click', 'a.dcs-toggle-table', (link, e) => {
    e.preventDefault();
    const content = link.parentElement!.nextElementSibling as HTMLElement;
    const type = link.getAttribute('data-type')!;
    if (content.style.display === 'none') {
      content.style.display = content.tagName.toLowerCase() === 'table' ? 'table' : 'block';
      link.textContent = `Click to collapse ${type}`;
    } else {
      content.style.display = 'none';
      link.textContent = `Click to see ${type}`;
    }
  });
}
