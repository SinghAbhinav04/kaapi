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
  | 'dashboard:read';
