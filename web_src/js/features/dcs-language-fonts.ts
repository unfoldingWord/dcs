import $ from 'jquery';
import lang_font_familiesJson from '../../../assets/lang_font_families.json';
import lang_font_linksJson from '../../../assets/lang_font_links.json';

const lang_font_families: {[key: string]: string[]} = lang_font_familiesJson as {[key: string]: string[]};
const lang_font_links: {[key: string]: string} = lang_font_linksJson as {[key: string]: string};

const set_dcs_fonts: any[] = [];
const set_dcs_selectors: any[] = [];

export function initDCSLanguageFonts() {
  $('[data-language]').each((_, tag) => {
    const lang = $(tag).attr('data-language');
    if (lang_font_families[lang]) {
      setDCSFontsHTML(lang_font_families[lang], `[data-language=${lang}], [data-language=${lang}] *`);
    }
  });
}

function setDCSFontsHTML(fonts: any[], selector: string) {
  if (set_dcs_selectors.includes(selector)) {
    return;
  }
  const $head = $('head');
  if (!fonts.includes('Noto Sans')) {
    fonts.push('Noto Sans');
  }
  for (const font of fonts) {
    if (!set_dcs_fonts.includes(font) && lang_font_links[font]) {
      const link = document.createElement('link');
      link.href = lang_font_links[font];
      link.rel = 'stylesheet';
      $head.append(link);
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
