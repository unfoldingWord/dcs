import {createTippy} from '../modules/tippy.ts';

export function initDCSValidationBadge() {
  const badges = document.querySelectorAll('.validation-message-badge');
  if (!badges) {
    return;
  }
  for (const badge of badges) {
    const tooltips = badge.querySelectorAll('.validation-message-tooltip');
    if (tooltips) {
      createTippy(badge, {
        trigger: 'mouseenter',
        allowHTML: true,
        content: 'Click to see status',
      });
      createTippy(badge, {
        trigger: 'click',
        allowHTML: true,
        maxWidth: 650,
        content: tooltips[0].innerHTML,
        interactive: true,
        hideOnClick: true,
      });
    }
  }
}
