import {createTippy} from '../modules/tippy.js';

export function initDCSInfoIcon() {
  const icon = document.getElementById('dcs-info-icon');
  if (!icon) {
    return;
  }
  const tooltip = document.getElementById('dcs-info-icon-tooltip');
  if (!tooltip) {
    return;
  }
  createTippy(icon, {
    allowHTML: true,
    maxWidth: 650,
    content: tooltip.innerHTML,
  });
}
