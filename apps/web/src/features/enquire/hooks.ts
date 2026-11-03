'use client';

import { useState } from 'react';

import { useCreateEnquiry } from '@/apis/enquiries';

import { emptyEnquiry, toEnquiryRequest, validateEnquiry } from './lib';
import type { EnquiryForm, EnquiryKind } from './types';

export function useEnquiryForm(initialKind: EnquiryKind) {
  const [form, setForm] = useState<EnquiryForm>(() => emptyEnquiry(initialKind));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const create = useCreateEnquiry();

  const update = <K extends keyof EnquiryForm>(key: K, value: EnquiryForm[K]) => setForm((f) => ({ ...f, [key]: value }));

  const submit = () => {
    const found = validateEnquiry(form);
    setErrors(found);
    if (Object.keys(found).length === 0) create.mutate(toEnquiryRequest(form));
  };

  return {
    form,
    update,
    submit,
    errors: { ...errors, ...(create.error?.fieldErrors ?? {}) },
    error: create.error && Object.keys(create.error.fieldErrors).length === 0 ? create.error : null,
    sending: create.isPending,
    reference: create.data?.ref ?? null,
    reset: () => {
      create.reset();
      setForm(emptyEnquiry(form.kind));
    },
  };
}
