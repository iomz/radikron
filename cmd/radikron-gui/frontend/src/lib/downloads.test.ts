import { describe, expect, it } from 'vitest';
import type { DownloadItem, Downloads } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';
import {
  DownloadStates,
  downloadDetail,
  downloadKey,
  downloadStateLabel,
  formatProgramTime,
  nextDownloads,
} from './downloads';

type ItemFields = Partial<Omit<DownloadItem, 'state'>> & { state?: string };
const item = (fields: ItemFields) =>
  ({ stationId: 'TBS', start: '20261012200000', stage: '', error: '', completed: 0, total: 0, ...fields }) as unknown as DownloadItem;

describe('download helpers', () => {
  it('builds the backend key', () => {
    expect(downloadKey('TBS', '20261012200000')).toBe('TBS|20261012200000');
  });

  it('formats radiko times and leaves other values alone', () => {
    expect(formatProgramTime('20261012203000')).toBe('10/12 20:30');
    expect(formatProgramTime('')).toBe('');
  });

  it('labels states', () => {
    expect(downloadStateLabel(DownloadStates.expired)).toBe('Expired');
    expect(downloadStateLabel('unknown')).toBe('unknown');
  });

  it('describes progress and failures', () => {
    expect(downloadDetail(item({ state: DownloadStates.downloading, stage: 'fetching playlist', completed: 3, total: 10 })))
      .toBe('fetching playlist (3/10)');
    expect(downloadDetail(item({ state: DownloadStates.failed, stage: 'encoding', error: 'ffmpeg not found' })))
      .toBe('encoding: ffmpeg not found');
    expect(downloadDetail(item({ state: DownloadStates.failed, error: 'HTTP 403' }))).toBe('HTTP 403');
    expect(downloadDetail(item({ state: DownloadStates.waiting, stage: 'ignored' }))).toBe('');
  });

  it('lists downloadable programs first and skips expired ones', () => {
    const downloads = {
      recent: [],
      scheduled: [
        item({ title: 'Gone', state: DownloadStates.expired }),
        item({ title: 'A', state: DownloadStates.available }),
        item({ title: 'B', state: DownloadStates.waiting }),
        item({ title: 'C', state: DownloadStates.waiting }),
      ],
    } as Downloads;
    expect(nextDownloads(downloads, 2).map((i) => i.title)).toEqual(['A', 'B']);
    expect(nextDownloads(null, 5)).toEqual([]);
  });
});
