export const ACCESS_PATHS = {
  overview: '/v1/staff/access',
  roles: '/v1/staff/roles',
  role: (id: string) => `/v1/staff/roles/${encodeURIComponent(id)}`,
  users: '/v1/staff/users',
  user: (id: string) => `/v1/staff/users/${encodeURIComponent(id)}`,
} as const;

export const accessKeys = {
  all: ['access'] as const,
  overview: () => [...accessKeys.all, 'overview'] as const,
  members: () => [...accessKeys.all, 'members'] as const,
};
