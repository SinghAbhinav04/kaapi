/** The one-click unsubscribe link in every offers email. */
export interface MarketingApi {
  unsubscribe(token: string): Promise<void>;
}
