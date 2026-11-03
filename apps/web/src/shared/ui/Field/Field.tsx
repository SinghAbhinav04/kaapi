'use client';

import { useId, type ReactNode } from 'react';

import { cx } from '@/shared/lib/cx';

import styles from './Field.module.css';
import type { CheckboxProps, SelectFieldProps, TextAreaFieldProps, TextFieldProps } from './types';

interface ShellProps {
  id: string;
  label: ReactNode;
  hint?: ReactNode;
  error?: string;
  optional?: boolean;
  children: ReactNode;
}

function Shell({ id, label, hint, error, optional, children }: ShellProps) {
  return (
    <div className={styles.field}>
      <label htmlFor={id}>
        {label} {optional && <span className={styles.optional}>Optional</span>}
      </label>
      {children}
      {error ? (
        <span id={`${id}-msg`} className={styles.error} role="alert">
          {error}
        </span>
      ) : (
        hint && (
          <span id={`${id}-msg`} className={styles.hint}>
            {hint}
          </span>
        )
      )}
    </div>
  );
}

const described = (id: string, hint: unknown, error?: string) => (error || hint ? `${id}-msg` : undefined);

export function TextField({ label, hint, error, optional, className, id: given, ...input }: TextFieldProps) {
  const auto = useId();
  const id = given ?? auto;
  return (
    <Shell id={id} label={label} hint={hint} error={error} optional={optional}>
      <input
        id={id}
        className={cx(styles.control, error && styles.invalid, className)}
        aria-invalid={error ? true : undefined}
        aria-describedby={described(id, hint, error)}
        {...input}
      />
    </Shell>
  );
}

export function SelectField({ label, hint, error, optional, options, className, id: given, ...select }: SelectFieldProps) {
  const auto = useId();
  const id = given ?? auto;
  return (
    <Shell id={id} label={label} hint={hint} error={error} optional={optional}>
      <select
        id={id}
        className={cx(styles.control, error && styles.invalid, className)}
        aria-invalid={error ? true : undefined}
        aria-describedby={described(id, hint, error)}
        {...select}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </Shell>
  );
}

export function TextAreaField({ label, hint, error, optional, className, id: given, ...area }: TextAreaFieldProps) {
  const auto = useId();
  const id = given ?? auto;
  return (
    <Shell id={id} label={label} hint={hint} error={error} optional={optional}>
      <textarea
        id={id}
        className={cx(styles.control, error && styles.invalid, className)}
        aria-invalid={error ? true : undefined}
        aria-describedby={described(id, hint, error)}
        {...area}
      />
    </Shell>
  );
}

export function Checkbox({ children, muted, className, ...input }: CheckboxProps) {
  return (
    <label className={cx(styles.checkbox, muted && styles.muted, className)}>
      <input type="checkbox" {...input} />
      <span>{children}</span>
    </label>
  );
}
