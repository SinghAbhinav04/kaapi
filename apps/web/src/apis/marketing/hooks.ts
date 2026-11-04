'use client';

import { useMutation } from '@tanstack/react-query';

import { marketingApi } from './marketing.api';

export const useUnsubscribe = () => useMutation({ mutationFn: (token: string) => marketingApi.unsubscribe(token) });
