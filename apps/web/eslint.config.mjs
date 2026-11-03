import next from 'eslint-config-next';

// Layering: app -> features -> apis/shared. Lower layers never import upper ones, so
// shared code stays reusable and features stay independent of each other's internals.
const restrict = (patterns) => ({ 'no-restricted-imports': ['error', { patterns }] });

export default [
  ...next,
  { ignores: ['.next/**', 'src/apis/generated/**', 'android/**', 'ios/**'] },
  {
    files: ['src/shared/**'],
    rules: restrict([{ group: ['@/features/*', '@/app/*', '@/apis/*'], message: 'shared/ must not depend on features, routes or the API layer.' }]),
  },
  {
    files: ['src/apis/**'],
    rules: restrict([{ group: ['@/features/*', '@/app/*'], message: 'apis/ must not depend on features or routes.' }]),
  },
  {
    files: ['src/features/**'],
    rules: restrict([
      { group: ['@/app/*'], message: 'features/ must not depend on routes.' },
      { group: ['@/features/*/*'], message: "Import another feature only through its index (e.g. '@/features/booking')." },
    ]),
  },
];
