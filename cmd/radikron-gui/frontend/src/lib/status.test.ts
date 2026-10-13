import { describe, expect, it } from 'vitest';
import { formatCheckTime, statusHeadline, statusShortLabel } from './status';

describe('formatCheckTime', () => {
  const now = new Date(2026, 9, 12, 9, 0);

  it('shows only the time for today', () => {
    const value = new Date(2026, 9, 12, 15, 30).toISOString();
    expect(formatCheckTime(value, now)).toBe(
      new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    );
  });

  it('adds the date for other days', () => {
    const value = new Date(2026, 9, 13, 1, 5).toISOString();
    const formatted = formatCheckTime(value, now);
    expect(formatted).toContain(new Date(value).toLocaleDateString([], { month: 'short', day: 'numeric' }));
    expect(formatted).toContain(new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
  });

  it('returns an empty string for missing or invalid times', () => {
    expect(formatCheckTime('', now)).toBe('');
    expect(formatCheckTime('not a time', now)).toBe('');
  });
});

describe('statusHeadline', () => {
  const now = new Date(2026, 9, 12, 14, 21);
  const at = (h: number, m: number) => new Date(2026, 9, 12, h, m).toISOString();
  const base = { starting: false, monitoring: true, checking: false, hasRules: true, lastCheck: at(14, 2), nextCheck: at(15, 0), configError: '' };
  const hhmm = (iso: string) => new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

  it('leads with the next check', () => {
    const { title, detail } = statusHeadline(base, now);
    expect(title).toBe(`Next check at ${hhmm(base.nextCheck)}`);
    expect(detail).toBe(`Last checked at ${hhmm(base.lastCheck)}. Radikron checks only while it is open.`);
  });

  it('covers the other states', () => {
    expect(statusHeadline({ ...base, starting: true }, now).title).toBe('Starting up…');
    expect(statusHeadline({ ...base, monitoring: false, configError: 'bad' }, now).title).toBe('Paused');
    expect(statusHeadline({ ...base, checking: true }, now).title).toBe('Checking for new programs…');
    expect(statusHeadline({ ...base, hasRules: false }, now).title).toBe('Nothing to download yet');
  });

  it('puts downloads in progress first', () => {
    expect(statusHeadline({ ...base, hasRules: false }, now, { downloading: ['Show'], pending: 1 }).title).toBe('Downloading Show');
    expect(statusHeadline({ ...base, checking: true }, now, { downloading: ['A', 'B'], pending: 2 }).title).toBe(
      'Downloading 2 programs',
    );
    expect(statusHeadline({ ...base, hasRules: false }, now, { downloading: [], pending: 1 }).title).toBe(
      `Next check at ${hhmm(base.nextCheck)}`,
    );
  });

  it('labels the sidebar state', () => {
    expect(statusShortLabel({ starting: false, monitoring: true })).toBe('Running');
    expect(statusShortLabel({ starting: false, monitoring: false })).toBe('Paused');
  });
});
