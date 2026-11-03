'use client';

import { useQuery } from '@tanstack/react-query';

import { operationsQueries } from './queries';

export const useOutbox = (limit?: number) => useQuery(operationsQueries.outbox(limit));

export const useDashboard = (enabled = true) => useQuery({ ...operationsQueries.dashboard(), enabled });
