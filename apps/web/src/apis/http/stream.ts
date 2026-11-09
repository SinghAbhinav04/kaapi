'use client';

import { useEffect, useEffectEvent } from 'react';

import { BROWSER_BASE_URL, STREAM_RETRY_MS } from './const';
import { refreshSession } from './session';

/**
 * Listens to a server-sent event stream for as long as the component is mounted.
 *
 * - `onReopen` runs whenever the stream comes back after a drop: whatever changed in the gap
 *   was never announced, so the caller refetches everything it follows.
 * - EventSource can't renew a session. When the server closes the stream (the 15-minute access
 *   token ran out on an idle page), the session is renewed before reconnecting; otherwise
 *   every retry would be refused and the page would go quietly stale.
 */
export function useEventStream(
  path: string,
  handlers: Record<string, (data: string) => void>,
  onReopen: () => void,
  enabled = true,
) {
  const handle = useEffectEvent((event: string, data: string) => handlers[event]?.(data));
  const reopened = useEffectEvent(onReopen);
  const events = Object.keys(handlers).join(',');

  useEffect(() => {
    if (!enabled) return;
    let source: EventSource | null = null;
    let retry = 0;
    let opened = false;
    let stopped = false;
    const open = () => {
      source = new EventSource(`${BROWSER_BASE_URL}${path}`);
      source.onopen = () => {
        if (opened) reopened();
        opened = true;
      };
      for (const name of events.split(',')) {
        source.addEventListener(name, (e) => handle(name, (e as MessageEvent<string>).data));
      }
      source.onerror = () => {
        if (source?.readyState !== EventSource.CLOSED) return; // the browser is reconnecting itself
        retry = window.setTimeout(async () => {
          await refreshSession().catch(() => false);
          if (!stopped) open();
        }, STREAM_RETRY_MS);
      };
    };
    open();
    return () => {
      stopped = true;
      window.clearTimeout(retry);
      source?.close();
    };
  }, [enabled, path, events]);
}
