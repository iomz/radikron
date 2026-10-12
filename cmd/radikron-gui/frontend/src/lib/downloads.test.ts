import { describe, expect, it } from 'vitest';
import type { DownloadItem, Downloads } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';
import {
  DownloadStates,
  downloadDetail,
  countByFilter,
  downloadsList,
  downloadKey,
  filterDownloads,
  downloadNote,
  downloadSource,
  formatAirTime,
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
      skipped: [],
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

describe('download rows', () => {
  const now = new Date('2026-10-12T05:21:00Z'); // Mon 14:21 in Japan

  it('formats air times relative to today in Japan time', () => {
    expect(formatAirTime('20261012130000', now)).toBe('13:00');
    expect(formatAirTime('20261013010000', now)).toBe('Tue 01:00');
    expect(formatAirTime('20261004220000', now)).toBe('10/04 22:00');
  });

  it('names the source and explains the state', () => {
    expect(downloadSource({ manual: true, ruleName: '' })).toBe('Manual');
    expect(downloadSource({ manual: false, ruleName: 'Jazz' })).toBe('Rule · Jazz');
    expect(downloadNote(item({ state: DownloadStates.waiting, start: '20261012200000', end: '20261012210000' }))).toBe('Airs 20:00–21:00');
    expect(downloadNote(item({ state: DownloadStates.available }))).toBe('Downloads at the next check');
    expect(downloadNote(item({ state: DownloadStates.failed, stage: 'encoding', error: 'ffmpeg not found' }))).toBe('encoding: ffmpeg not found');
    expect(downloadNote(item({ state: DownloadStates.expired }))).toBe('No longer on radiko');
  });
});

describe('download filters', () => {
  const items = [
    item({ state: DownloadStates.waiting }),
    item({ state: DownloadStates.waiting }),
    item({ state: DownloadStates.failed }),
  ];

  it('counts and filters by state', () => {
    const counts = countByFilter(items);
    expect(counts.all).toBe(3);
    expect(counts.waiting).toBe(2);
    expect(counts.failed).toBe(1);
    expect(counts.expired).toBe(0);
    expect(filterDownloads(items, 'failed')).toHaveLength(1);
    expect(filterDownloads(items, 'all')).toHaveLength(3);
  });
});

describe('downloadsList', () => {
  it('appends this session\'s downloads that left the schedule', () => {
    const list = downloadsList({
      scheduled: [item({ title: 'A', start: '1' })],
      recent: [
        item({ title: 'Done', start: '2', state: DownloadStates.completed }),
        item({ title: 'A failed earlier', start: '1', state: DownloadStates.failed }),
        item({ title: 'A done', start: '1', state: DownloadStates.completed }),
      ],
    } as Downloads);
    expect(list.map((i) => i.title)).toEqual(['A', 'Done']);
  });
});
