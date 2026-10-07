import {registerGlobalInitFunc} from '../modules/observer.ts';
import {addDelegatedEventListener} from '../utils/dom.ts';

// Submits the catalog search form when a sort radio is picked, mirroring
// upstream's initRepositorySearch (repo-search.ts). We register our own init
// function because upstream's requires a .repo-search-filter-reset element
// that the catalog form does not have.
export function initDCSCatalogSearch() {
  registerGlobalInitFunc('initDCSCatalogSearch', (form: HTMLFormElement) => {
    addDelegatedEventListener(form, 'change', 'input[type="radio"]', () => form.submit());
  });
}
