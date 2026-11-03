/**
 * Session storage that never throws: private windows and blocked storage just behave
 * like an empty store. Used only for per-tab conveniences (e.g. this tab's hold token).
 */
export const sessionStore = {
  get<T>(key: string): T | null {
    try {
      const raw = window.sessionStorage.getItem(key);
      return raw ? (JSON.parse(raw) as T) : null;
    } catch {
      return null;
    }
  },
  set(key: string, value: unknown): void {
    try {
      window.sessionStorage.setItem(key, JSON.stringify(value));
    } catch {
      // storage unavailable: the flow still works within this page
    }
  },
  remove(key: string): void {
    try {
      window.sessionStorage.removeItem(key);
    } catch {
      // ignore
    }
  },
};
