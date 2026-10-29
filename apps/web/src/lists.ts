/** Quick-add parsing from the Add item design (IMP-4): name and quantity share one field. */
const UNITS = 'cans?|tins?|packs?|packets?|bottles?|bags?|boxes|box|bunch(?:es)?|spools?|dozen|loaf|loaves|jars?|lbs?|kg|g|ml|l|oz|cups?|tbsp|tsp';
const LEADING = new RegExp(`^(x?\\d+(?:[.,/]\\d+)?\\s*(?:${UNITS})?)\\s+(.+)$`, 'i');
const TRAILING = new RegExp(`^(.+?)\\s+(x\\d+|\\d+(?:[.,/]\\d+)?\\s*(?:${UNITS}))$`, 'i');

export function parseQuickAdd(line: string): { name: string; quantityText: string } {
  const text = line.trim().replace(/\s+/g, ' ');
  const lead = LEADING.exec(text);
  if (lead) return { name: lead[2], quantityText: lead[1].replace(/(\d)\s+/, '$1 ').trim() };
  const trail = TRAILING.exec(text);
  if (trail) return { name: trail[1], quantityText: trail[2].replace(/(\d)(?=[a-z])/i, '$1 ') };
  return { name: text, quantityText: '' };
}

export const LIST_COLOURS = ['#c62f55', '#b83a2b', '#e0701a', '#8a6a00', '#5e9626', '#0e7c86', '#3c8de0', '#8e4cc0'];
