import {buildSearchQuery, parseSearchQuery} from './dcs-search-builder.ts';

const fields = ['keyword', 'lang', 'subject', 'book'];

test('parseSearchQuery', () => {
  const values = parseSearchQuery('tn, lang:en, fr, subject: Bible ,unknown:x, book:mat', fields);
  expect(values.get('keyword')).toEqual(['tn']);
  expect(values.get('lang')).toEqual(['en', 'fr']);
  expect(values.get('subject')).toEqual(['Bible', 'unknown:x']);
  expect(values.get('book')).toEqual(['mat']);
  expect(parseSearchQuery('', fields).get('keyword')).toEqual([]);
});

test('buildSearchQuery', () => {
  const values = new Map([['keyword', ['tn', '']], ['lang', ['en', 'fr']], ['subject', []], ['book', ['mat']]]);
  expect(buildSearchQuery(values)).toBe('tn, lang:en, lang:fr, book:mat');
  expect(parseSearchQuery(buildSearchQuery(values), fields)).toEqual(new Map([['keyword', ['tn']], ['lang', ['en', 'fr']], ['subject', []], ['book', ['mat']]]));
});
