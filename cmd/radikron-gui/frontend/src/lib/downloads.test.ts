import { describe, expect, it } from 'vitest';
import {
  DownloadItem,
  DownloadState,
  Downloads,
} from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';
import { downloadDetail, downloadKey, downloadStateLabel, formatProgramTime, nextDownloads } from './downloads';

const item = (fields: Partial<DownloadItem>) => new DownloadItem({ stationId: 'TBS', start: '20261012200000', ...fields });

describe('download helpers', () => {
  it('builds the backend key', () => {
    expect(downloadKey('TBS', '20261012200000')).toBe('TBS|20261012200000');
  });

  it('formats radiko times and leaves other values alone', () => {
    expect(formatProgramTime('20261012203000')).toBe('10/12 20:30');
    expect(formatProgramTime('')).toBe('');
  });

  it('labels states', () => {
    expect(downloadStateLabel(DownloadState.DownloadExpired)).toBe('Expired');
    expect(downloadStateLabel('unknown')).toBe('unknown');
  });

  it('describes progress and failures', () => {
    expect(downloadDetail(item({ state: DownloadState.DownloadDownloading, stage: 'fetching playlist', completed: 3, total: 10 })))
      .toBe('fetching playlist (3/10)');
    expect(downloadDetail(item({ state: DownloadState.DownloadFailed, stage: 'encoding', error: 'ffmpeg not found' })))
      .toBe('encoding: ffmpeg not found');
    expect(downloadDetail(item({ state: DownloadState.DownloadFailed, error: 'HTTP 403' }))).toBe('HTTP 403');
    expect(downloadDetail(item({ state: DownloadState.DownloadWaiting, stage: 'ignored' }))).toBe('');
  });

  it('lists downloadable programs first and skips expired ones', () => {
    const downloads = new Downloads({
      scheduled: [
        item({ title: 'Gone', state: DownloadState.DownloadExpired }),
        item({ title: 'A', state: DownloadState.DownloadAvailable }),
        item({ title: 'B', state: DownloadState.DownloadWaiting }),
        item({ title: 'C', state: DownloadState.DownloadWaiting }),
      ],
    });
    expect(nextDownloads(downloads, 2).map((i) => i.title)).toEqual(['A', 'B']);
    expect(nextDownloads(null, 5)).toEqual([]);
  });
});
