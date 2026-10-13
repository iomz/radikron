import type { DownloadItem } from '../../bindings/github.com/iomz/radikron/cmd/radikron-gui/models';

// radiko's broadcast day runs from 05:00 to 29:00 (05:00 the next morning),
// in Japan time. Times are the YYYYMMDDHHmmss strings radiko uses.
export const DAY_START_HOUR = 5;
export const DAY_HOURS = 24;

const pattern = /^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})\d{2}$/;

function minutesSinceEpochDay(value: string): number | null {
  const m = pattern.exec(value);
  if (!m) return null;
  const day = Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3])) / 60000;
  return day + Number(m[4]) * 60 + Number(m[5]);
}

// broadcastDay returns the broadcast day a radiko time belongs to, as
// "YYYYMMDD": programs before 05:00 belong to the previous day.
export function broadcastDay(value: string): string | null {
  const minutes = minutesSinceEpochDay(value);
  if (minutes === null) return null;
  const shifted = new Date((minutes - DAY_START_HOUR * 60) * 60000);
  const y = shifted.getUTCFullYear();
  const mo = String(shifted.getUTCMonth() + 1).padStart(2, '0');
  const d = String(shifted.getUTCDate()).padStart(2, '0');
  return `${y}${mo}${d}`;
}

// todayBroadcastDay is the broadcast day of a moment in Japan time.
export function todayBroadcastDay(now: Date): string {
  const jst = new Date(now.getTime() + 9 * 3600_000);
  const value =
    `${jst.getUTCFullYear()}${String(jst.getUTCMonth() + 1).padStart(2, '0')}${String(jst.getUTCDate()).padStart(2, '0')}` +
    `${String(jst.getUTCHours()).padStart(2, '0')}${String(jst.getUTCMinutes()).padStart(2, '0')}00`;
  return broadcastDay(value)!;
}

// position returns how far into broadcast day `day` a time is, from 0 to 100.
export function position(value: string, day: string): number | null {
  const minutes = minutesSinceEpochDay(value);
  const start = minutesSinceEpochDay(`${day}${String(DAY_START_HOUR).padStart(2, '0')}0000`);
  if (minutes === null || start === null) return null;
  return ((minutes - start) / (DAY_HOURS * 60)) * 100;
}

export interface Block {
  item: DownloadItem;
  left: number;
  width: number;
}

export interface Lane {
  stationId: string;
  blocks: Block[];
}

// The narrowest block, in percent of the day, so short programs stay visible.
const MIN_WIDTH = 1.2;

// buildLanes places the programs of broadcast day `day` on one lane per
// station, ordered by each station's first program.
export function buildLanes(items: DownloadItem[], day: string): Lane[] {
  const lanes = new Map<string, Block[]>();
  for (const item of items) {
    if (!item.end || broadcastDay(item.start) !== day) continue;
    const from = position(item.start, day);
    const to = position(item.end, day);
    if (from === null || to === null) continue;
    const left = Math.max(0, from);
    const width = Math.max(MIN_WIDTH, Math.min(100, to) - left);
    const blocks = lanes.get(item.stationId) ?? [];
    blocks.push({ item, left, width });
    lanes.set(item.stationId, blocks);
  }
  return [...lanes.entries()]
    .map(([stationId, blocks]) => ({ stationId, blocks: blocks.sort((a, b) => a.left - b.left) }))
    .sort((a, b) => a.blocks[0].left - b.blocks[0].left);
}

// scheduledDays lists the broadcast days that have programs, in order.
export function scheduledDays(items: DownloadItem[]): string[] {
  const days = new Set<string>();
  for (const item of items) {
    if (!item.end) continue;
    const day = broadcastDay(item.start);
    if (day) days.add(day);
  }
  return [...days].sort();
}

// formatDay formats "YYYYMMDD" as "Mon 10/12".
export function formatDay(day: string): string {
  const date = new Date(Date.UTC(Number(day.slice(0, 4)), Number(day.slice(4, 6)) - 1, Number(day.slice(6, 8))));
  const weekday = date.toLocaleDateString('en-US', { weekday: 'short', timeZone: 'UTC' });
  return `${weekday} ${day.slice(4, 6)}/${day.slice(6, 8)}`;
}
