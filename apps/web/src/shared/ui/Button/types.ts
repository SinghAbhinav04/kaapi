import type { AnchorHTMLAttributes, ButtonHTMLAttributes, ReactNode } from 'react';

import type { BUTTON_SIZES, BUTTON_VARIANTS } from './const';

export type ButtonVariant = (typeof BUTTON_VARIANTS)[number];
export type ButtonSize = (typeof BUTTON_SIZES)[number];

interface CommonProps {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Pill-shaped (navigation, secondary) instead of the squarer action shape. */
  pill?: boolean;
  block?: boolean;
  loading?: boolean;
  icon?: ReactNode;
}

export interface ButtonProps extends CommonProps, ButtonHTMLAttributes<HTMLButtonElement> {}

export interface ButtonLinkProps extends CommonProps, Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> {
  href: string;
  prefetch?: boolean;
}
