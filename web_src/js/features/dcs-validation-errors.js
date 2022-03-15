import $ from 'jquery';

export function initDcsValidationErrors() {
  const invalidButtonEl = $('.validation-errors-trigger');

  if (!invalidButtonEl.length) {
    return;
  }

  invalidButtonEl.removeAttr('href'); // intended for noscript mode only
  invalidButtonEl.popup({
    position: 'bottom center',
    hoverable: true,
  });
}
