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
