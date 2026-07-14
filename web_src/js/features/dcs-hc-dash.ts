export function initDCSHealthcheckDashboard() {
  const form = document.querySelector<HTMLFormElement>('#hc-dash-form');
  if (!form) return;

  for (const el of form.querySelectorAll<HTMLInputElement>('.js-hc-dash-autosubmit')) {
    el.addEventListener('change', () => {
      form.requestSubmit();
    });
  }
}
