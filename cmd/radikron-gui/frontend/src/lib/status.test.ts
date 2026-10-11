import { describe, expect, it } from 'vitest';
import { formatCheckTime } from './status';

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
