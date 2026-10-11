// formatCheckTime formats an RFC 3339 time from the backend Status for
// display: the time of day, preceded by the date when it is not today.
// It returns "" for an empty or invalid value.
export function formatCheckTime(value: string, now: Date = new Date()): string {
  if (!value) {
    return '';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  const time = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  if (date.toDateString() === now.toDateString()) {
    return time;
  }
  return `${date.toLocaleDateString([], { month: 'short', day: 'numeric' })} ${time}`;
}

interface StatusFields {
  starting: boolean;
  monitoring: boolean;
  checking: boolean;
  hasRules: boolean;
  lastCheck: string;
  nextCheck: string;
  configError: string;
}

// statusHeadline phrases the status the way a person would say it: the main
// line answers "what happens next", the detail adds when it last checked.
export function statusHeadline(status: StatusFields, now: Date = new Date()): { title: string; detail: string } {
  const last = formatCheckTime(status.lastCheck, now);
  const lastText = last ? `Last checked at ${last}. ` : '';
  const openText = 'Radikron checks only while it is open.';
  if (status.starting) {
    return { title: 'Starting up…', detail: 'Loading your configuration.' };
  }
  if (!status.monitoring) {
    return {
      title: 'Paused',
      detail: status.configError ? 'The configuration did not load. Fix it, then retry.' : 'Radikron is not checking for programs.',
    };
  }
  if (status.checking) {
    return { title: 'Checking for new programs…', detail: lastText + openText };
  }
  if (!status.hasRules) {
    return { title: 'Nothing to download yet', detail: 'Add a rule with a title, performer, or keyword to start.' };
  }
  const next = formatCheckTime(status.nextCheck, now);
  return { title: next ? `Next check at ${next}` : 'Waiting for the next check', detail: lastText + openText };
}

// statusShortLabel is the one-word state for the sidebar.
export function statusShortLabel(status: Pick<StatusFields, 'starting' | 'monitoring'>): string {
  if (status.starting) return 'Starting…';
  return status.monitoring ? 'Running' : 'Paused';
}
