'use client';

import { useQuery } from '@tanstack/react-query';

import { analyticsQueries } from './queries';
import type { AnalyticsParams } from './types';

export const useAnalytics = (params: AnalyticsParams) => useQuery(analyticsQueries.report(params));
