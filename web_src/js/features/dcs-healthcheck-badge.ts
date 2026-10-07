import {GET} from '../modules/fetch.ts';

const {appSubUrl} = window.config;

function getIssueTitles(issues: Record<string, unknown>): string[] {
  const titles: string[] = [];

  for (const value of Object.values(issues)) {
    if (!Array.isArray(value) || value.length === 0) continue;
    const issue = value[0];
    if (!issue || typeof issue !== 'object') continue;
    const negativeTitle = (issue as {negative_title?: unknown}).negative_title;
    if (typeof negativeTitle !== 'string' || !negativeTitle) continue;
    if (!titles.includes(negativeTitle)) titles.push(negativeTitle);
  }

  return titles;
}

function buildTooltipContent(payload: unknown): string {
  if (!payload || typeof payload !== 'object') return 'Healthcheck details unavailable';

  const response = payload as {ok?: unknown; error?: unknown; data?: unknown};
  if (!response.ok || !response.data || typeof response.data !== 'object') {
    if (typeof response.error === 'string' && response.error) return response.error;
    return 'Healthcheck details unavailable';
  }

  const data = response.data as {overall_severity_level?: unknown; issues?: unknown};
  const severity = typeof data.overall_severity_level === 'string' ? data.overall_severity_level : 'unknown';

  if (!data.issues || typeof data.issues !== 'object') {
    return `Healthcheck: ${severity}`;
  }

  const titles = getIssueTitles(data.issues as Record<string, unknown>);
  if (titles.length === 0) return `Healthcheck: ${severity}`;

  const maxItems = 4;
  const shown = titles.slice(0, maxItems);
  const suffix = titles.length > maxItems ? '; ...' : '';
  return `Healthcheck: ${severity}; ${shown.join('; ')}${suffix}`;
}

async function loadTooltipContent(el: HTMLElement): Promise<void> {
  const loadedState = el.getAttribute('data-hc-loaded');
  if (loadedState === 'true' || loadedState === 'loading') return;

  const owner = el.getAttribute('data-hc-owner');
  const repo = el.getAttribute('data-hc-repo');
  if (!owner || !repo) return;

  el.setAttribute('data-hc-loaded', 'loading');

  const ref = el.getAttribute('data-hc-ref');
  const refQuery = ref ? `?ref=${encodeURIComponent(ref)}` : '';
  const url = `${appSubUrl}/api/v1/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/healthcheck${refQuery}`;

  let content = 'Healthcheck details unavailable';
  try {
    const response = await GET(url);
    const payload = await response.json();
    content = buildTooltipContent(payload);
  } catch {
    // Keep fallback content.
  }

  el.setAttribute('data-tooltip-content', content);
  (el as any)._tippy?.setContent(content);
  el.setAttribute('data-hc-loaded', 'true');
}

export function initDCSHealthcheckBadges() {
  for (const el of document.querySelectorAll<HTMLElement>('.js-healthcheck-badge')) {
    if (el.getAttribute('data-hc-init') === 'true') continue;
    el.setAttribute('data-hc-init', 'true');

    if (!el.getAttribute('data-hc-owner') || !el.getAttribute('data-hc-repo')) continue;

    const load = () => {
      loadTooltipContent(el);
    };
    el.addEventListener('mouseenter', load);
    el.addEventListener('focus', load);
  }
}
