import $ from 'jquery';

export function initDCSValidationErrors() {
  const invalidButtonEl = $('.validation-message-trigger');

  if (!invalidButtonEl.length) {
    return;
  }

  invalidButtonEl.popup({
    on: 'click',
    hoverable: false,
    closable: false,
    position: 'bottom center',
    lastResort: 'bottom center',
    name: 'validation-message',
    movePopup: false,
    debug: true,
  });
}
