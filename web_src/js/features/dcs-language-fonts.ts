import lang_font_familiesJson from '../../../assets/lang_font_families.json';
import lang_font_linksJson from '../../../assets/lang_font_links.json';

const lang_font_families: {[key: string]: string[]} = lang_font_familiesJson;
const lang_font_links: {[key: string]: string} = lang_font_linksJson;

const set_dcs_fonts: string[] = [];
const set_dcs_selectors: string[] = [];

export function initDCSLanguageFonts() {
  for (const tag of document.querySelectorAll('[data-language]')) {
    const lang = tag.getAttribute('data-language')!;
    if (lang_font_families[lang]) {
      setDCSFontsHTML(lang_font_families[lang], `[data-language=${lang}], [data-language=${lang}] *`);
    }
  }
}

function setDCSFontsHTML(fonts: string[], selector: string) {
  if (set_dcs_selectors.includes(selector)) {
    return;
  }
  if (!fonts.includes('Noto Sans')) {
    fonts.push('Noto Sans');
  }
  for (const font of fonts) {
    if (!set_dcs_fonts.includes(font) && lang_font_links[font]) {
      const link = document.createElement('link');
      link.href = lang_font_links[font];
      link.rel = 'stylesheet';
      document.head.append(link);
      set_dcs_fonts.push(font);
    }
  }
  document.head.insertAdjacentHTML('beforeend', `
<style type="text/css">
    ${selector} {
    font-family: "${fonts.join(', ')}, sans-serif" !important;
  };
</style>`);
  set_dcs_selectors.push(selector);
}
