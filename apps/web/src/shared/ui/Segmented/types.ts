export interface SegmentOption<V extends string> {
  value: V;
  label: string;
}

export interface SegmentedProps<V extends string> {
  label: string;
  options: readonly SegmentOption<V>[];
  value: V;
  onChange: (value: V) => void;
  tone?: 'light' | 'dark';
}
