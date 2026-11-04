import type { CampaignRequest, CampaignResponse, CampaignsResponse, OfferRequest, OfferResponse, OffersResponse } from './types';

/** Offers: promo codes and automatic festive or seasonal discounts (permission promos:write). */
export interface OffersApi {
  list(signal?: AbortSignal): Promise<OffersResponse>;
  create(body: OfferRequest): Promise<OfferResponse>;
  update(id: string, body: OfferRequest): Promise<OfferResponse>;
  /** Offer and seasonal emails to guests who opted in, and how many guests that is. */
  campaigns(signal?: AbortSignal): Promise<CampaignsResponse>;
  createCampaign(body: CampaignRequest): Promise<CampaignResponse>;
}
