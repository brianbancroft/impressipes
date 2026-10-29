import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** Sticker fills in a fixed order, so the same id always gets the same colour. */
export const STICKERS = ['bg-butter', 'bg-mint', 'bg-lilac', 'bg-tangerine', 'bg-sky', 'bg-rose'] as const;

export function pick<T>(items: readonly T[], seed: string): T {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) | 0;
  return items[Math.abs(h) % items.length];
}

/** A stored list/category colour softened into a pastel that sticker-ink reads on. */
export const tint = (colour: string, amount = 38) => `color-mix(in oklab, ${colour} ${amount}%, #fff)`;
