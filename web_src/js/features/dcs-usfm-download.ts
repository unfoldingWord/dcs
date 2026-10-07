import {addDelegatedEventListener} from '../utils/dom.ts';

declare global {
  // eslint-disable-next-line @typescript-eslint/consistent-type-definitions -- declaration merging requires an interface
  interface Window {
    // defined by public/assets/js-dcs/usfm-alignment-remover.js, only included on Aligned Bible pages
    downloadUnalignedUSFM?: (owner: string, repo: string, ref: string, statusElement?: HTMLElement | null) => void;
  }
}

// Download links for unaligned USFM on the releases, tags and branches pages.
// Delegated listeners because inline onclick handlers are blocked by the CSP
// script nonce policy.
export function initDCSUsfmDownload() {
  addDelegatedEventListener<HTMLAnchorElement, MouseEvent>(document, 'click', 'a.dcs-usfm-download', (link, e) => {
    e.preventDefault();
    if (!window.downloadUnalignedUSFM) return;
    const statusElement = link.parentElement!.querySelector<HTMLElement>('.dcs-usfm-download-status');
    window.downloadUnalignedUSFM(
      link.getAttribute('data-owner')!,
      link.getAttribute('data-repo')!,
      link.getAttribute('data-ref')!,
      statusElement,
    );
  });
}
