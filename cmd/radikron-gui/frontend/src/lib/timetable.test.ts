import { describe, expect, it } from 'vitest';
import type { DownloadItem } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';
import { broadcastDay, buildLanes, formatDay, position, scheduledDays, todayBroadcastDay } from './timetable';

const item = (stationId: string, start: string, end: string) =>
  ({ stationId, start, end, title: `${stationId} ${start}`, state: 'waiting' }) as unknown as DownloadItem;

describe('timetable', () => {
  it('assigns early-morning programs to the previous broadcast day', () => {
    expect(broadcastDay('20261012130000')).toBe('20261012');
    expect(broadcastDay('20261013010000')).toBe('20261012');
    expect(broadcastDay('20261013050000')).toBe('20261013');
    expect(broadcastDay('bad')).toBeNull();
  });

  it('knows the current broadcast day in Japan time', () => {
    expect(todayBroadcastDay(new Date('2026-10-12T05:21:00Z'))).toBe('20261012'); // 14:21 JST
    expect(todayBroadcastDay(new Date('2026-10-12T17:00:00Z'))).toBe('20261012'); // 02:00 JST next day
  });

  it('positions times across the 05:00–29:00 day', () => {
    expect(position('20261012050000', '20261012')).toBe(0);
    expect(position('20261012170000', '20261012')).toBe(50);
    expect(position('20261013050000', '20261012')).toBe(100);
  });

  it('builds one lane per station for the chosen day', () => {
    const lanes = buildLanes(
      [
        item('LFR', '20261012200000', '20261012210000'),
        item('TBS', '20261012130000', '20261012140000'),
        item('TBS', '20261013010000', '20261013020000'),
        item('QRR', '20261013130000', '20261013140000'),
      ],
      '20261012',
    );
    expect(lanes.map((l) => l.stationId)).toEqual(['TBS', 'LFR']);
    expect(lanes[0].blocks).toHaveLength(2);
    expect(lanes[0].blocks[0].left).toBeCloseTo(33.33, 1);
    expect(lanes[0].blocks[0].width).toBeCloseTo(4.17, 1);
  });

  it('lists days with programs and formats them', () => {
    expect(scheduledDays([item('TBS', '20261013010000', '20261013020000'), item('QRR', '20261013130000', '20261013140000')])).toEqual([
      '20261012',
      '20261013',
    ]);
    expect(formatDay('20261012')).toBe('Mon 10/12');
  });
});
