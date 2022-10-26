import $ from 'jquery';
import lang_font_families from '../../../assets/lang_font_families.json';
import lang_font_links from '../../../assets/lang_font_links.json';

const set_dcs_fonts = [];
const set_dcs_selectors = [];

export function initDCSLanguageFonts() {
  $.each($('[data-language]'), (_, tag) => {
    const lang = $(tag).attr('data-language');
    if(lang_font_families[lang]) {
      setDCSFontsHTML(lang_font_families[lang], `[data-language=${lang}], [data-language=${lang}] *`);
    }
  });
}

export function setDCSFontsHTML(fonts, selector) {
  if (set_dcs_selectors.includes(selector))
    return;
  const $head = $("head");
  if (!fonts.includes("Noto Sans"))
    fonts.push("Noto Sans");
  fonts.forEach(font => {
    if (!set_dcs_fonts.includes(font) && lang_font_links[font]) {
      $head.append(`<link href="${lang_font_links[font]}" rel="stylesheet">`);
      set_dcs_fonts.push(font);
    }
});
  $head.append(`
<style type="text/css">
    ${selector} {
    font-family: "${fonts.join(", ")}, sans-serif" !important;
  }; 
</style>`);
  set_dcs_selectors.push(selector);
}