import { describe, expect, it } from 'vitest';
import * as radikron from '../../bindings/github.com/iomz/radikron/models';
import type { DownloadItem } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';
import { describeRule, timestampRuleName, ruleActivity, ruleProblem } from './rules';

const rule = (fields: Partial<radikron.Rule>) => radikron.Rule.createFrom({ Name: 'r', ...fields });
const label = (id: string) => ({ FMT: 'TOKYO FM' })[id] ?? id;

describe('rules', () => {
  it('describes what a rule matches', () => {
    expect(
      describeRule(
        rule({ Title: 'MIDDAY LOUNGE', StationID: 'FMT', DoW: ['sun', 'mon'], Window: '48h', Exclude: radikron.Criteria.createFrom({ Pfm: 'GUEST' }) }),
        label,
      ),
    ).toEqual(['Title has “MIDDAY LOUNGE”', 'on TOKYO FM', 'Mon, Sun', 'past 2 days', 'skips “GUEST”']);
    expect(describeRule(rule({ Keyword: 'jazz', Window: '5h' }), label)).toEqual(['Mentions “jazz”', 'past 5h']);
  });

  it('names a new rule after when it was created', () => {
    const now = new Date(2026, 9, 12, 15, 50, 39);
    expect(timestampRuleName([], now)).toBe('rule-20261012155039');
    expect(timestampRuleName([{ Name: 'rule-20261012155039' }], now)).toBe('rule-20261012155039-2');
  });

  it('explains why a draft cannot be saved', () => {
    const rules = [rule({ Name: 'Show', Title: 'Show' })];
    expect(ruleProblem(rule({ Name: ' ', Title: 'x' }), rules, null)).toMatch(/name/);
    expect(ruleProblem(rule({ Name: 'Show', Title: 'x' }), rules, null)).toMatch(/already exists/);
    expect(ruleProblem(rule({ Name: 'Show', Title: 'x' }), rules, 'Show')).toBeNull();
    expect(ruleProblem(rule({ Name: 'New', StationID: 'FMT' }), rules, null)).toMatch(/title, host, or keyword/);
  });

  it('finds the downloads a rule still has to make', () => {
    const item = (start: string, state: string, ruleName = 'Show', manual = false) =>
      ({ stationId: 'FMT', start, end: start, title: start, state, ruleName, manual }) as unknown as DownloadItem;
    const activity = ruleActivity('Show', [
      item('20261013130000', 'waiting'),
      item('20261012130000', 'available'),
      item('20261011130000', 'completed'),
      item('20261010130000', 'waiting', 'Other'),
      item('20261009130000', 'waiting', 'Show', true),
    ]);
    expect(activity.upcoming).toBe(2);
    expect(activity.next?.start).toBe('20261012130000');
  });
});
