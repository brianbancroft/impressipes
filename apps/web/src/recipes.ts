import type { Ingredient, Recipe, RecipeSummary } from './api';

const FRACTIONS: [number, string][] = [[0.125, '⅛'], [0.25, '¼'], [1 / 3, '⅓'], [0.5, '½'], [2 / 3, '⅔'], [0.75, '¾']];

/** 1.5 -> "1 ½", 0.33 -> "⅓", 2.04 -> "2". Falls back to up to 2 decimals. */
export function formatQuantity(n: number): string {
  const whole = Math.floor(n + 0.02);
  const frac = n - whole;
  if (frac < 0.03) return String(whole || 0);
  const hit = FRACTIONS.find(([v]) => Math.abs(v - frac) < 0.03);
  if (hit) return whole ? `${whole} ${hit[1]}` : hit[1];
  return String(Math.round(n * 100) / 100);
}

export function scaleIngredient(i: Ingredient, factor: number): string {
  if (i.quantity == null) return i.rawText ?? i.item;
  const parts = [formatQuantity(i.quantity * factor), i.unit, i.item].filter(Boolean);
  return parts.join(' ') + (i.prepNote ? `, ${i.prepNote}` : '');
}

export function totalMinutes(r: Pick<RecipeSummary, 'prepMinutes' | 'cookMinutes'>): number {
  return (r.prepMinutes ?? 0) + (r.cookMinutes ?? 0);
}

export function formatMinutes(m: number): string {
  if (m < 60) return `${m} min`;
  const h = Math.floor(m / 60);
  return m % 60 ? `${h} h ${m % 60} min` : `${h} h`;
}

export function formatTimer(sec: number): string {
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
}

export function sourceLine(r: Recipe, memberName?: (id: string) => string | undefined): string {
  const by = memberName?.(r.createdBy);
  let src: string | null = null;
  try {
    src = r.sourceUrl ? new URL(r.sourceUrl).hostname.replace(/^www\./, '') : null;
  } catch {}
  return [src ? `From ${src}` : r.sourceKind === 'ai' ? 'Added by AI' : null, by ? `added by ${by}` : null]
    .filter(Boolean)
    .join(', ');
}

/** Splits an ingredient for the quantity badge: { qty: "1 ½", unit: "cup", text: "flour, sifted" }. */
export function ingredientParts(i: Ingredient, factor: number): { qty?: string; unit?: string; text: string } {
  if (i.quantity == null) return { text: i.rawText ?? i.item };
  return { qty: formatQuantity(i.quantity * factor), unit: i.unit || undefined, text: i.item + (i.prepNote ? `, ${i.prepNote}` : '') };
}

/** "2 L" -> { qty: "2", unit: "L" }; "a few" -> { qty: "a few" }. Items with no quantity read as 1. */
export function splitQuantity(text: string): { qty: string; unit?: string } {
  const t = text.trim();
  if (!t) return { qty: '1' };
  const m = /^x?(\d+(?:[.,/]\d+)?)\s*(.*)$/i.exec(t);
  return m ? { qty: m[1], unit: m[2] || undefined } : { qty: t };
}
