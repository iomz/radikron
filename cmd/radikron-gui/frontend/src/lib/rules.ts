import type { Rule } from '../../bindings/github.com/iomz/radikron/models';
import type { DownloadItem } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';
import { DownloadStates } from './downloads';

// weekdays are the rule day values in display order.
export const weekdays = [
  { id: 'mon', label: 'Mon' },
  { id: 'tue', label: 'Tue' },
  { id: 'wed', label: 'Wed' },
  { id: 'thu', label: 'Thu' },
  { id: 'fri', label: 'Fri' },
  { id: 'sat', label: 'Sat' },
  { id: 'sun', label: 'Sun' },
] as const;

// windowOptions limit how far back a rule reaches for past episodes. A rule
// skips programs that started longer ago than its window.
export const windowOptions = [
  { value: '', label: 'Whole week on radiko' },
  { value: '24h', label: 'Last 24 hours' },
  { value: '48h', label: 'Last 2 days' },
  { value: '72h', label: 'Last 3 days' },
] as const;

function quoted(text: string): string {
  return `“${text}”`;
}

// describeRule summarizes what a rule matches, e.g.
// ["Title has “MIDDAY LOUNGE”", "on TOKYO FM", "Sun", "past 2 days"].
export function describeRule(rule: Rule, stationLabel: (id: string) => string): string[] {
  const parts: string[] = [];
  if (rule.Title) parts.push(`Title has ${quoted(rule.Title)}`);
  if (rule.Pfm) parts.push(`Host has ${quoted(rule.Pfm)}`);
  if (rule.Keyword) parts.push(`Mentions ${quoted(rule.Keyword)}`);
  if (rule.StationID) parts.push(`on ${stationLabel(rule.StationID)}`);
  const days = weekdays.filter((d) => rule.DoW?.some((v) => v.toLowerCase() === d.id));
  if (days.length > 0 && days.length < 7) parts.push(days.map((d) => d.label).join(', '));
  if (rule.Window) {
    const preset = windowOptions.find((w) => w.value === rule.Window);
    parts.push(preset ? preset.label.toLowerCase().replace('last ', 'past ') : `past ${rule.Window}`);
  }
  const skips = [rule.Exclude?.Title, rule.Exclude?.Pfm, rule.Exclude?.Keyword].filter(Boolean) as string[];
  if (skips.length > 0) parts.push(`skips ${skips.map(quoted).join(', ')}`);
  return parts;
}

// timestampRuleName names a new rule after when it was created, like
// "rule-20261012155039" in local time, so it can be saved without naming it
// first and still be told apart later.
export function timestampRuleName(rules: Pick<Rule, 'Name'>[], now: Date = new Date()): string {
  const p = (n: number) => String(n).padStart(2, '0');
  const base = `rule-${now.getFullYear()}${p(now.getMonth() + 1)}${p(now.getDate())}${p(now.getHours())}${p(now.getMinutes())}${p(now.getSeconds())}`;
  let name = base;
  for (let i = 2; rules.some((r) => r.Name === name); i++) name = `${base}-${i}`;
  return name;
}

// ruleProblem explains why a rule draft cannot be saved, or returns null.
// original is the rule's name before editing, null for a new rule.
export function ruleProblem(rule: Rule, rules: Rule[], original: string | null): string | null {
  const name = rule.Name.trim();
  if (!name) return 'Give the rule a name.';
  if (name !== original && rules.some((r) => r.Name === name)) return `A rule named ${quoted(name)} already exists.`;
  if (!rule.Title.trim() && !rule.Pfm.trim() && !rule.Keyword.trim()) return 'Fill in a title, host, or keyword to match.';
  return null;
}

const pending: string[] = [DownloadStates.waiting, DownloadStates.available, DownloadStates.downloading];

// ruleActivity counts a rule's downloads still to come and finds the next one.
export function ruleActivity(name: string, items: DownloadItem[]): { upcoming: number; next: DownloadItem | null } {
  const mine = items.filter((i) => !i.manual && i.ruleName === name && pending.includes(i.state)).sort((a, b) => a.start.localeCompare(b.start));
  return { upcoming: mine.length, next: mine[0] ?? null };
}
