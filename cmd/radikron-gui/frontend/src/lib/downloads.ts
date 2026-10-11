import {
  DownloadState,
  type DownloadItem,
  type Downloads,
} from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

// downloadKey identifies a program the way the backend does: a station
// cannot air two programs that start at the same time.
export function downloadKey(stationID: string, start: string): string {
  return `${stationID}|${start}`;
}

const stateLabels: Record<string, string> = {
  [DownloadState.DownloadWaiting]: 'Waiting',
  [DownloadState.DownloadAvailable]: 'Available',
  [DownloadState.DownloadDownloading]: 'Downloading',
  [DownloadState.DownloadCompleted]: 'Completed',
  [DownloadState.DownloadFailed]: 'Failed',
  [DownloadState.DownloadExpired]: 'Expired',
};

const stateDescriptions: Record<string, string> = {
  [DownloadState.DownloadWaiting]: 'Not available yet: the program has not finished airing.',
  [DownloadState.DownloadAvailable]: 'Available on radiko; the next check downloads it.',
  [DownloadState.DownloadDownloading]: 'Downloading now.',
  [DownloadState.DownloadCompleted]: 'Downloaded.',
  [DownloadState.DownloadFailed]: 'The last download attempt failed.',
  [DownloadState.DownloadExpired]: 'No longer available on radiko.',
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
  if (item.state === DownloadState.DownloadFailed) {
    return [item.stage, item.error].filter(Boolean).join(': ');
  }
  if (item.state === DownloadState.DownloadDownloading && item.stage) {
    return item.total > 0 ? `${item.stage} (${item.completed}/${item.total})` : item.stage;
  }
  return '';
}

// nextDownloads returns the scheduled programs that can still be
// downloaded, soonest first.
export function nextDownloads(downloads: Downloads | null, limit: number): DownloadItem[] {
  return (downloads?.scheduled ?? [])
    .filter((item) => item.state !== DownloadState.DownloadExpired)
    .slice(0, limit);
}
