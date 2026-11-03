export interface StepperProps {
  label: string;
  value: number;
  min?: number;
  max?: number;
  onChange: (value: number) => void;
  tone?: 'light' | 'dark';
  /** Highlights the control, e.g. when the value is at a limit the API enforces. */
  warn?: boolean;
}
