export const PANELS = [
  { value: 'people', label: 'People' },
  { value: 'roles', label: 'Roles' },
] as const;
export type Panel = (typeof PANELS)[number]['value'];

/** A role's team decides whose lane it works and the title in reply signatures. */
export const TEAM_OPTIONS = [
  { value: 'RESERVATIONS', label: 'Reservations' },
  { value: 'SALES', label: 'Events & Sales' },
  { value: 'GM', label: 'General Manager' },
] as const;

export const TEAM_LABEL: Record<string, string> = {
  RESERVATIONS: 'Reservations',
  SALES: 'Events & Sales',
  GM: 'General Manager',
  ADMIN: 'Administrator',
  GUEST: 'Guest',
};

export const MIN_PASSWORD = 12;
export const GENERATED_PASSWORD_LENGTH = 16;

export const COPY = {
  title: 'Team & access',
  description: 'Create staff accounts and decide exactly what each role can see and do. Changes apply within seconds.',
  teamHint: 'Which lane this role works and the title in reply signatures.',
  systemRole: 'Built in and fixed: the Administrator always has every permission, including new ones.',
  passwordHint: 'At least 12 characters. Share it privately; they can be given a new one any time.',
  signOutNote: 'Changing someone’s role, deactivating them or resetting their password signs them out everywhere.',
} as const;
