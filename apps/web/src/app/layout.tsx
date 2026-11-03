import type { Metadata, Viewport } from 'next';
import type { ReactNode } from 'react';

import { SITE } from '@/shared/const';
import { fontVariables } from '@/shared/styles/fonts';

import { Providers } from './providers';
import '@/shared/styles/index.css';

export const metadata: Metadata = {
  title: { default: `${SITE.name} · Coffee-estate resort, Chikkamagaluru`, template: `%s · ${SITE.name}` },
  description: '64 rooms and 12 villas on a working coffee estate. Book direct for the best rate, with GST shown before you pay.',
  applicationName: SITE.name,
};

export const viewport: Viewport = {
  themeColor: '#f5f1ea',
  width: 'device-width',
  initialScale: 1,
  viewportFit: 'cover',
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en-IN" className={fontVariables} data-scroll-behavior="smooth">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
