export const OFFERS_PATHS = {
  offers: '/v1/staff/offers',
  offer: (id: string) => `/v1/staff/offers/${encodeURIComponent(id)}`,
  campaigns: '/v1/staff/campaigns',
} as const;

export const offersKeys = {
  all: ['offers'] as const,
  list: () => [...offersKeys.all, 'list'] as const,
  campaigns: () => [...offersKeys.all, 'campaigns'] as const,
};
