import type { OfferRequest, OfferResponse, OffersResponse } from './types';

/** Offers: promo codes and automatic festive or seasonal discounts (permission promos:write). */
export interface OffersApi {
  list(signal?: AbortSignal): Promise<OffersResponse>;
  create(body: OfferRequest): Promise<OfferResponse>;
  update(id: string, body: OfferRequest): Promise<OfferResponse>;
}
