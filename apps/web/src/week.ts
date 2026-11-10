/** The household plans weeks in America/Vancouver, Sunday through Saturday. */
export const FAMILY_TZ = 'America/Vancouver';

export type Slot = 'breakfast' | 'lunch' | 'dinner' | 'extra';

export const SLOTS: { id: Slot; label: string; add: string }[] = [
  { id: 'breakfast', label: 'Breakfast', add: '+ add' },
  { id: 'lunch', label: 'Lunch', add: '+ add' },
  { id: 'dinner', label: 'Dinner', add: '+ add' },
  { id: 'extra', label: 'Extra', add: '+ add' },
];

/** Civil date YYYY-MM-DD in the family timezone. */
export function isoDate(d: Date, tz = FAMILY_TZ): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: tz, year: 'numeric', month: '2-digit', day: '2-digit' }).format(d);
}

export function todayISO(): string {
  return isoDate(new Date());
}

/** Treat a civil date as a UTC midnight so day arithmetic does not depend on the browser zone. */
function utcDate(iso: string): Date {
  const [y, m, d] = iso.split('-').map(Number);
  return new Date(Date.UTC(y, (m || 1) - 1, d || 1));
}

export function addDays(iso: string, n: number): string {
  const dt = utcDate(iso);
  dt.setUTCDate(dt.getUTCDate() + n);
  const y = dt.getUTCFullYear();
  const m = String(dt.getUTCMonth() + 1).padStart(2, '0');
  const day = String(dt.getUTCDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

/** 0 = Sunday … 6 = Saturday. */
export function weekdayIndex(iso: string): number {
  return utcDate(iso).getUTCDay();
}

export function startOfWeek(iso: string): string {
  return addDays(iso, -weekdayIndex(iso));
}

export function weekDates(sunday: string): string[] {
  return Array.from({ length: 7 }, (_, i) => addDays(sunday, i));
}

const shortWeekday = new Intl.DateTimeFormat('en-US', { weekday: 'short', timeZone: 'UTC' });
const longWeekday = new Intl.DateTimeFormat('en-US', { weekday: 'long', timeZone: 'UTC' });

export function dayHeading(iso: string): { dow: string; num: string; long: string } {
  const dt = utcDate(iso);
  return { dow: shortWeekday.format(dt), num: String(dt.getUTCDate()), long: longWeekday.format(dt) };
}

export function rangeLabel(from: string, to: string): string {
  const fmt = new Intl.DateTimeFormat('en-US', { weekday: 'short', month: 'short', day: 'numeric', timeZone: 'UTC' });
  return `${fmt.format(utcDate(from))} – ${fmt.format(utcDate(to))}`;
}

/** A cook timestamp is an instant; show the calendar day in the family timezone. */
export function cookedOn(iso: string): string {
  return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric', timeZone: FAMILY_TZ }).format(new Date(iso));
}
