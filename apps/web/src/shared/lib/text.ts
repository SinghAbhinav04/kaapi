/** "Arjun Menon" → "AM"; one word → its first two letters. */
export function initials(name: string | null | undefined): string {
  const words = (name ?? '').trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return '?';
  if (words.length === 1) return (words[0] ?? '').slice(0, 2).toUpperCase();
  return `${words[0]?.[0] ?? ''}${words[words.length - 1]?.[0] ?? ''}`.toUpperCase();
}

/** plural(2, 'night') → "2 nights"; plural(1, 'child', 'children') → "1 child". */
export function plural(count: number, one: string, many = `${one}s`): string {
  return `${count} ${count === 1 ? one : many}`;
}

export function firstName(name: string | null | undefined): string {
  return (name ?? '').trim().split(/\s+/)[0] ?? '';
}

/** "2 adults, 1 child" */
export function formatParty(adults: number, children: number): string {
  const parts = [plural(adults, 'adult')];
  if (children > 0) parts.push(plural(children, 'child', 'children'));
  return parts.join(', ');
}
