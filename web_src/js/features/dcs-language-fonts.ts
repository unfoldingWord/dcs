import $ from 'jquery';
import lang_font_families from '../../../assets/lang_font_families.json';
import lang_font_links from '../../../assets/lang_font_links.json';
import {html} from '../utils/html.ts';

const fontFamilies: Record<string, string[]> = lang_font_families as Record<string, string[]>;
const fontLinks: Record<string, string> = lang_font_links as Record<string, string>;

const set_dcs_fonts: string[] = [];
const set_dcs_selectors: string[] = [];

export function initDCSLanguageFonts() {
  $('[data-language]').each((_, tag) => {
    const lang = $(tag).attr('data-language');
    if (lang && fontFamilies[lang]) {
      setDCSFontsHTML(fontFamilies[lang], `[data-language=${lang}], [data-language=${lang}] *`);
    }
  });
}

function setDCSFontsHTML(fonts: string[], selector: string) {
  if (set_dcs_selectors.includes(selector)) {
    return;
  }
  const $head = $('head');
  if (!fonts.includes('Noto Sans')) {
    fonts.push('Noto Sans');
  }
  for (const font of fonts) {
    if (!set_dcs_fonts.includes(font) && fontLinks[font]) {
      $head.append(html`<link href="${fontLinks[font]}" rel="stylesheet">`);
      set_dcs_fonts.push(font);
    }
  }
  $head.append(`
<style type="text/css">
    ${selector} {
    font-family: "${fonts.join(', ')}, sans-serif" !important;
  };
</style>`);
  set_dcs_selectors.push(selector);
}
