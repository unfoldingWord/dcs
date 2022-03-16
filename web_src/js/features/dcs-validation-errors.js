export function initDcsValidationErrors() {
  const invalidButtonEl = $('.validation-message-trigger');

  if (!invalidButtonEl.length) {
    return;
  }

  invalidButtonEl.popup({
    on: 'click',
    hoverable: false,
    closable: false,
    position: 'bottom center',
    lastResort: 'bottom right',
    name: 'validation-message',
  });
}
