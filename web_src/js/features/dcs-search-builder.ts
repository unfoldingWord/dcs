import {registerGlobalInitFunc} from '../modules/observer.ts';
import {toggleElem} from '../utils/dom.ts';

// The free-text field: its values carry no "field:" prefix in the query
const keywordField = 'keyword';

// Mirrors models/door43metadata.ParseRepoSearchKeyword: tokens are comma separated and a
// "field:" prefix switches the field for itself and the following unprefixed tokens.
export function parseSearchQuery(query: string, fields: string[]): Map<string, string[]> {
  const values = new Map<string, string[]>(fields.map((field) => [field, []]));
  let current = keywordField;
  for (const token of query.split(',')) {
    let value = token.trim();
    for (const field of fields) {
      if (value.startsWith(`${field}:`)) {
        current = field;
        value = value.slice(field.length + 1).trim();
        break;
      }
    }
    if (value) values.get(current)!.push(value);
  }
  return values;
}

export function buildSearchQuery(values: Map<string, string[]>): string {
  const tokens: string[] = [];
  for (const [field, fieldValues] of values) {
    for (const value of fieldValues) {
      if (value) tokens.push(field === keywordField ? value : `${field}:${value}`);
    }
  }
  return tokens.join(', ');
}

export function initDCSSearchBuilder() {
  registerGlobalInitFunc('initDCSSearchBuilder', (el: HTMLElement) => {
    const query = el.parentElement!.querySelector<HTMLInputElement>('input[name="q"]');
    const inputs = Array.from(el.querySelectorAll<HTMLInputElement>('input[data-field]'));
    if (!query || !inputs.length) return;
    const fields = inputs.map((input) => input.getAttribute('data-field')!);
    const toggle = el.querySelector<HTMLButtonElement>('.dcs-search-builder-toggle')!;

    const setExpanded = (expanded: boolean) => {
      toggle.setAttribute('aria-expanded', String(expanded));
      toggleElem(el.querySelector('.dcs-search-builder-fields')!, expanded);
      toggleElem(el.querySelector('.dcs-search-builder-collapsed')!, !expanded);
      toggleElem(el.querySelector('.dcs-search-builder-expanded')!, expanded);
    };
    toggle.addEventListener('click', () => setExpanded(toggle.getAttribute('aria-expanded') !== 'true'));

    const fillFromQuery = () => {
      const values = parseSearchQuery(query.value, fields);
      for (const input of inputs) input.value = values.get(input.getAttribute('data-field')!)!.join(', ');
      return values;
    };
    query.addEventListener('input', fillFromQuery);
    for (const input of inputs) {
      input.addEventListener('input', () => {
        query.value = buildSearchQuery(new Map(inputs.map((i) => [i.getAttribute('data-field')!, i.value.split(',').map((v) => v.trim())])));
      });
      input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') query.form!.requestSubmit();
      });
    }
    // open the builder when the page loaded with field filters in the query
    const initial = fillFromQuery();
    initial.delete(keywordField);
    if (initial.values().some((v) => v.length)) setExpanded(true);
  });
}
