import {createTippy} from '../modules/tippy.ts';

export function initDCSInfoIcon() {
  const icon = document.querySelector('#dcs-info-icon');
  if (!icon) {
    return;
  }
  const tooltip = document.querySelector('#dcs-info-icon-tooltip');
  if (!tooltip) {
    return;
  }
  createTippy(icon, {
    allowHTML: true,
    maxWidth: 650,
    content: tooltip.innerHTML,
  });
}
