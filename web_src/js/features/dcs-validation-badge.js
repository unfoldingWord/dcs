import {createTippy} from '../modules/tippy.js';

export function initDCSValidationBadge() {
  const badges = document.getElementsByClassName('validation-message-badge');
  if (!badges) {
    return;
  }
  [].forEach.call(badges, (badge) => {
    const tooltips = badge.getElementsByClassName('validation-message-tooltip');
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
      });
    }
  });
}
