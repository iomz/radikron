import { describe, expect, it } from 'vitest';
import { parseRadikoTime, programAction } from './programs';

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
