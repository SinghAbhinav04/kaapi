import type { ReactNode } from 'react';

export interface EmptyStateProps {
  title: string;
  children?: ReactNode;
  action?: ReactNode;
  compact?: boolean;
}

export interface ErrorStateProps {
  title?: string;
  /** A message safe to show (the API's errors are written for people). */
  message: string;
  /** Shown so a guest or staff member can quote it when asking for help. */
  requestId?: string;
  onRetry?: () => void;
  retrying?: boolean;
  compact?: boolean;
}
