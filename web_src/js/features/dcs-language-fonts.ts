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
      setDCSFontsHTML(lang_font_families[lang], `[data-language="${CSS.escape(lang)}"], [data-language="${CSS.escape(lang)}"] *`);
    }
  }
}

function setDCSFontsHTML(fonts: string[], selector: string) {
  if (set_dcs_selectors.includes(selector)) {
    return;
  }
  const fontFamilies = [...fonts];
  if (!fontFamilies.includes('Noto Sans')) {
    fontFamilies.push('Noto Sans');
  }
  for (const font of fontFamilies) {
    if (!set_dcs_fonts.includes(font) && lang_font_links[font]) {
      const link = document.createElement('link');
      link.href = lang_font_links[font];
      link.rel = 'stylesheet';
      document.head.append(link);
      set_dcs_fonts.push(font);
    }
  }
  const style = document.createElement('style');
  style.textContent = `${selector} { font-family: ${fontFamilies.map((font) => `"${font.replaceAll('"', '\\"')}"`).join(', ')}, sans-serif !important; }`;
  document.head.append(style);
  set_dcs_selectors.push(selector);
}
