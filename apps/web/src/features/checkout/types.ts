export interface GuestForm {
  name: string;
  email: string;
  phone: string;
  country: string;
  arrivalWindow: string;
  specialRequests: string;
  consent: boolean;
  marketing: boolean;
}

/** Keyed like the API's validation details ("guest.email", "consent", …). */
export type FormErrors = Partial<Record<string, string>>;
