type ClassValue = string | false | null | undefined;

/** Joins class names, skipping falsy ones: cx(styles.card, active && styles.active). */
export function cx(...values: ClassValue[]): string {
  return values.filter(Boolean).join(' ');
}
