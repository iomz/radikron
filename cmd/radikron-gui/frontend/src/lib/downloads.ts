import type {
  DownloadItem,
  Downloads,
} from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

// Download states as the backend reports them (DownloadState in Go). They are
// repeated here so this module does not load the Wails runtime, which the
// generated models import.
export const DownloadStates = {
  waiting: 'waiting',
  available: 'available',
  downloading: 'downloading',
  completed: 'completed',
  failed: 'failed',
  expired: 'expired',
} as const;

// downloadKey identifies a program the way the backend does: a station
// cannot air two programs that start at the same time.
export function downloadKey(stationID: string, start: string): string {
  return `${stationID}|${start}`;
}

const stateLabels: Record<string, string> = {
  [DownloadStates.waiting]: 'Waiting',
  [DownloadStates.available]: 'Available',
  [DownloadStates.downloading]: 'Downloading',
  [DownloadStates.completed]: 'Completed',
  [DownloadStates.failed]: 'Failed',
  [DownloadStates.expired]: 'Expired',
};

const stateDescriptions: Record<string, string> = {
  [DownloadStates.waiting]: 'Not available yet: the program has not finished airing.',
  [DownloadStates.available]: 'Available on radiko; the next check downloads it.',
  [DownloadStates.downloading]: 'Downloading now.',
  [DownloadStates.completed]: 'Downloaded.',
  [DownloadStates.failed]: 'The last download attempt failed.',
  [DownloadStates.expired]: 'No longer available on radiko.',
};

export function downloadStateLabel(state: string): string {
  return stateLabels[state] ?? state;
}

export function downloadStateDescription(state: string): string {
  return stateDescriptions[state] ?? '';
}

// formatProgramTime formats radiko's YYYYMMDDHHmmss time as "MM/DD HH:mm".
export function formatProgramTime(value: string): string {
  if (!/^\d{14}$/.test(value)) {
    return value;
  }
  return `${value.slice(4, 6)}/${value.slice(6, 8)} ${value.slice(8, 10)}:${value.slice(10, 12)}`;
}

// downloadDetail describes progress or failure, such as
// "fetching playlist (3/10)" or "encoding: ffmpeg not found".
export function downloadDetail(item: DownloadItem): string {
  if (item.state === DownloadStates.failed) {
    return [item.stage, item.error].filter(Boolean).join(': ');
  }
  if (item.state === DownloadStates.downloading && item.stage) {
    return item.total > 0 ? `${item.stage} (${item.completed}/${item.total})` : item.stage;
  }
  return '';
}

// nextDownloads returns the scheduled programs that can still be
// downloaded, soonest first.
export function nextDownloads(downloads: Downloads | null, limit: number): DownloadItem[] {
  return (downloads?.scheduled ?? [])
    .filter((item) => item.state !== DownloadStates.expired)
    .slice(0, limit);
}

const JST = 'Asia/Tokyo';

function jstDay(date: Date): string {
  return date.toLocaleDateString('en-US', { timeZone: JST, year: 'numeric', month: '2-digit', day: '2-digit' });
}

// formatAirTime formats a radiko start time (Japan time) as "13:00" for
// today, "Mon 01:00" within a week, or "10/04 22:00" otherwise.
export function formatAirTime(start: string, now: Date = new Date()): string {
  if (!/^\d{14}$/.test(start)) {
    return start;
  }
  const time = `${start.slice(8, 10)}:${start.slice(10, 12)}`;
  const date = new Date(`${start.slice(0, 4)}-${start.slice(4, 6)}-${start.slice(6, 8)}T${time}:00+09:00`);
  if (jstDay(date) === jstDay(now)) {
    return time;
  }
  const days = Math.abs(date.getTime() - now.getTime()) / 86_400_000;
  if (days < 6) {
    return `${date.toLocaleDateString('en-US', { timeZone: JST, weekday: 'short' })} ${time}`;
  }
  return `${start.slice(4, 6)}/${start.slice(6, 8)} ${time}`;
}

// downloadSource names where a download came from.
export function downloadSource(item: Pick<DownloadItem, 'manual' | 'ruleName'>): string {
  if (item.manual) return 'Manual';
  return item.ruleName ? `Rule · ${item.ruleName}` : 'Rule';
}

// downloadNote is the one-line explanation under a download.
export function downloadNote(item: DownloadItem): string {
  const time = (v: string) => (/^\d{14}$/.test(v) ? `${v.slice(8, 10)}:${v.slice(10, 12)}` : '');
  switch (item.state) {
    case DownloadStates.downloading:
      return downloadDetail(item) || 'Downloading';
    case DownloadStates.available:
      return 'The next check downloads it';
    case DownloadStates.waiting:
      return item.end ? `Airs ${time(item.start)}–${time(item.end)}` : 'Not aired yet';
    case DownloadStates.failed:
      return downloadDetail(item) || 'Download failed';
    case DownloadStates.expired:
      return 'No longer on radiko';
    case DownloadStates.completed:
      return 'Downloaded';
    default:
      return '';
  }
}

// downloadFilters are the Downloads screen filters, in display order.
export const downloadFilters = [
  { id: 'all', label: 'All' },
  { id: DownloadStates.waiting, label: 'Waiting' },
  { id: DownloadStates.available, label: 'Available' },
  { id: DownloadStates.downloading, label: 'Downloading' },
  { id: DownloadStates.failed, label: 'Failed' },
  { id: DownloadStates.expired, label: 'Expired' },
] as const;

export type DownloadFilter = (typeof downloadFilters)[number]['id'];

// countByFilter counts the items each filter shows.
export function countByFilter(items: DownloadItem[]): Record<DownloadFilter, number> {
  const counts = Object.fromEntries(downloadFilters.map((f) => [f.id, 0])) as Record<DownloadFilter, number>;
  counts.all = items.length;
  for (const item of items) {
    if (item.state in counts) {
      counts[item.state as DownloadFilter] += 1;
    }
  }
  return counts;
}

export function filterDownloads(items: DownloadItem[], filter: DownloadFilter): DownloadItem[] {
  return filter === 'all' ? items : items.filter((item) => item.state === filter);
}
