import type {
  AvailabilityParams,
  AvailabilityResponse,
  CalendarParams,
  CalendarResponse,
  FxRatesResponse,
  PropertyResponse,
  RoomResponse,
  RoomsResponse,
} from './types';

/** Public catalogue: the resort, its rooms, nightly prices and what's bookable. */
export interface CatalogApi {
  getProperty(signal?: AbortSignal): Promise<PropertyResponse>;
  listRooms(signal?: AbortSignal): Promise<RoomsResponse>;
  getRoom(slug: string, signal?: AbortSignal): Promise<RoomResponse>;
  getCalendar(slug: string, params: CalendarParams, signal?: AbortSignal): Promise<CalendarResponse>;
  searchAvailability(params: AvailabilityParams, signal?: AbortSignal): Promise<AvailabilityResponse>;
  /** Rates for the "≈" estimate in a guest's currency; bookings are always charged in INR. */
  getFxRates(signal?: AbortSignal): Promise<FxRatesResponse>;
}
