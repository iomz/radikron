import { describe, expect, it } from 'vitest';
import { filterResults, parseRadikoTime, programAction, suggestRuleName } from './programs';

describe('program helpers', () => {
  it('parses radiko times as Japan time', () => {
    expect(parseRadikoTime('20261012200000')?.toISOString()).toBe('2026-10-12T11:00:00.000Z');
    expect(parseRadikoTime('2026-10-12')).toBeNull();
  });

  it('chooses the action from the air time', () => {
    const now = new Date('2026-10-12T03:00:00Z'); // 12:00 in Japan
    expect(programAction('20261012200000', '20261012210000', now)).toBe('schedule');
    expect(programAction('20261012113000', '20261012115800', now)).toBe('schedule');
    expect(programAction('20261011200000', '20261011210000', now)).toBe('download');
    expect(programAction('20261005110000', '20261005120000', now)).toBe('expired');
  });
});

describe('suggestRuleName', () => {
  const prog = { StationID: 'TBS', Title: '週末シティポップ', Pfm: 'Host', Desc: 'jazz and pop' };
  it('picks the first matching rule with criteria', () => {
    expect(
      suggestRuleName(
        [
          { Name: 'Other station', StationID: 'QRR', Title: 'シティポップ' },
          { Name: 'No criteria', StationID: 'TBS' },
          { Name: 'Jazz', Keyword: 'jazz' },
        ],
        prog,
      ),
    ).toBe('Jazz');
    expect(suggestRuleName([{ Name: 'Rock', Keyword: 'rock' }], prog)).toBe('');
  });
});

describe('filterResults', () => {
  const now = new Date('2026-10-12T05:21:00Z'); // Mon 14:21 in Japan
  const day = (ft: string) => (Number(ft.slice(8, 10)) < 5 ? '20261012' : ft.slice(0, 8));
  const progs = [
    { Ft: '20261001100000', To: '20261001110000', id: 'expired' },
    { Ft: '20261012090000', To: '20261012100000', id: 'morning' },
    { Ft: '20261012200000', To: '20261012210000', id: 'evening' },
    { Ft: '20261013010000', To: '20261013020000', id: 'late' },
    { Ft: '20261013100000', To: '20261013110000', id: 'tomorrow' },
  ];
  const ids = (filter: Parameters<typeof filterResults>[1]) => filterResults(progs, filter, day, now).map((p) => p.id);

  it('drops programs radiko no longer has', () => {
    expect(ids({ day: '', part: '' })).toEqual(['morning', 'evening', 'late', 'tomorrow']);
  });

  it('filters by broadcast day and part of day', () => {
    expect(ids({ day: '20261012', part: '' })).toEqual(['morning', 'evening', 'late']);
    expect(ids({ day: '20261012', part: 'late' })).toEqual(['late']);
    expect(ids({ day: '', part: 'morning' })).toEqual(['morning', 'tomorrow']);
  });
});
