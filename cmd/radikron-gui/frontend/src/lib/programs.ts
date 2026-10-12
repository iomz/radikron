// radiko times are YYYYMMDDHHmmss in Japan time (UTC+9, no daylight saving).
export function parseRadikoTime(value: string): Date | null {
  const m = /^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})$/.exec(value);
  if (!m) {
    return null;
  }
  const date = new Date(`${m[1]}-${m[2]}-${m[3]}T${m[4]}:${m[5]}:${m[6]}+09:00`);
  return Number.isNaN(date.getTime()) ? null : date;
}

// How long after the end a recording becomes available, and how long radiko
// keeps it after the start; these mirror the backend.
const AVAILABLE_AFTER_END_MS = 5 * 60 * 1000;
const ARCHIVE_RETENTION_MS = 7 * 24 * 60 * 60 * 1000;

// ProgramAction is what Radikron can do with a program found in Program
// Search: download its recording now, schedule it for when the recording is
// available, or nothing because radiko no longer has it.
export type ProgramAction = 'download' | 'schedule' | 'expired';

export function programAction(ft: string, to: string, now: Date = new Date()): ProgramAction {
  const start = parseRadikoTime(ft);
  const end = parseRadikoTime(to);
  if (start && now.getTime() - start.getTime() > ARCHIVE_RETENTION_MS) {
    return 'expired';
  }
  if (end && now.getTime() >= end.getTime() + AVAILABLE_AFTER_END_MS) {
    return 'download';
  }
  return 'schedule';
}

interface RuleLike {
  Name?: string;
  StationID?: string;
  Title?: string;
  Pfm?: string;
  Keyword?: string;
}

interface ProgramLike {
  StationID: string;
  Title: string;
  Pfm?: string;
  Desc?: string;
}

// suggestRuleName picks the first rule that would match the program, so its
// folder is preselected when the program is downloaded by hand. It returns ""
// when none matches.
export function suggestRuleName(rules: RuleLike[], prog: ProgramLike): string {
  const match = rules.find((rule) => {
    if (rule.StationID && rule.StationID !== prog.StationID) return false;
    if (rule.Title && !prog.Title.includes(rule.Title)) return false;
    if (rule.Pfm && rule.Pfm !== prog.Pfm) return false;
    if (rule.Keyword && !prog.Title.includes(rule.Keyword) && !prog.Desc?.includes(rule.Keyword)) return false;
    return Boolean(rule.Title || rule.Pfm || rule.Keyword);
  });
  return match?.Name ?? '';
}

// Parts of the broadcast day for the Search time filter, by broadcast hour
// (radiko counts 00:00–04:59 as 24–28 of the previous day).
export const dayParts = [
  { id: 'morning', label: 'Morning (5–12)', from: 5, to: 12 },
  { id: 'afternoon', label: 'Afternoon (12–18)', from: 12, to: 18 },
  { id: 'evening', label: 'Evening (18–24)', from: 18, to: 24 },
  { id: 'late', label: 'Late night (24–29)', from: 24, to: 29 },
] as const;

export type DayPart = (typeof dayParts)[number]['id'];

function broadcastHour(ft: string): number {
  const hour = Number(ft.slice(8, 10));
  return hour < 5 ? hour + 24 : hour;
}

interface ResultFilter {
  // Broadcast day "YYYYMMDD", or "" for any day.
  day: string;
  part: DayPart | '';
}

// filterResults keeps the programs Radikron can still download that air on
// the chosen broadcast day and part of the day.
export function filterResults<T extends { Ft: string; To: string }>(
  progs: T[],
  filter: ResultFilter,
  dayOf: (ft: string) => string | null,
  now: Date = new Date(),
): T[] {
  const part = dayParts.find((p) => p.id === filter.part);
  return progs.filter((prog) => {
    if (programAction(prog.Ft, prog.To, now) === 'expired') return false;
    if (filter.day && dayOf(prog.Ft) !== filter.day) return false;
    if (part) {
      const hour = broadcastHour(prog.Ft);
      if (hour < part.from || hour >= part.to) return false;
    }
    return true;
  });
}
