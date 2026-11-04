export type {
  AuthResponse,
  MeResponse,
  OtpRequestResponse,
  OtpVerifyRequest,
  StaffLoginRequest,
  User,
} from '@/apis/generated/schema';

export type Role = 'GUEST' | 'RESERVATIONS' | 'SALES' | 'GM' | 'ADMIN';

/** Mirrors the API's permission names (apps/api/internal/auth/permissions.go). */
export type Permission =
  | 'own_bookings:read'
  | 'bookings:read'
  | 'bookings:export'
  | 'rates:read'
  | 'rates:write'
  | 'inventory:write'
  | 'promos:write'
  | 'enquiries:read'
  | 'enquiries:read_all_lanes'
  | 'enquiries:reroute'
  | 'enquiries:send'
  | 'outbox:read'
  | 'facts:write'
  | 'dashboard:read'
  | 'retention:run'
  | 'integrations:manage'
  | 'users:manage'
  | 'analytics:read';

/** Staff sign-in step 1's answer: a code went to sent_to (masked). */
export type StaffChallenge = { code_sent: boolean; sent_to: string };
