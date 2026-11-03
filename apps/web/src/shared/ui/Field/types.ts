import type { InputHTMLAttributes, ReactNode, SelectHTMLAttributes, TextareaHTMLAttributes } from 'react';

interface FieldMeta {
  label: ReactNode;
  /** Shown under the control; replaced by `error` when there is one. */
  hint?: ReactNode;
  error?: string;
  optional?: boolean;
}

export interface TextFieldProps extends FieldMeta, Omit<InputHTMLAttributes<HTMLInputElement>, 'children'> {}

export interface SelectFieldProps extends FieldMeta, SelectHTMLAttributes<HTMLSelectElement> {
  options: readonly { value: string; label: string }[];
}

export interface TextAreaFieldProps extends FieldMeta, TextareaHTMLAttributes<HTMLTextAreaElement> {}

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  children: ReactNode;
  muted?: boolean;
}
