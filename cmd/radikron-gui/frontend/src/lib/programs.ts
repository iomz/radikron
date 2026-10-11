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
