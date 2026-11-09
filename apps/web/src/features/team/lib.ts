import { GENERATED_PASSWORD_LENGTH } from './const';

const ALPHABET = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789';

/** A random starting password without look-alike characters. */
export function generatePassword(length = GENERATED_PASSWORD_LENGTH): string {
  // Bytes at or above the last whole multiple of the alphabet are thrown away, so every
  // character is equally likely (a plain modulo favours the first few).
  const limit = 256 - (256 % ALPHABET.length);
  let out = '';
  while (out.length < length) {
    for (const b of crypto.getRandomValues(new Uint8Array(length))) {
      if (b < limit && out.length < length) out += ALPHABET[b % ALPHABET.length];
    }
  }
  return out;
}

/** Groups items by key, keeping first-seen order. */
export function groupBy<T>(items: readonly T[], key: (t: T) => string): [string, T[]][] {
  const map = new Map<string, T[]>();
  for (const item of items) {
    const k = key(item);
    map.set(k, [...(map.get(k) ?? []), item]);
  }
  return [...map.entries()];
}
