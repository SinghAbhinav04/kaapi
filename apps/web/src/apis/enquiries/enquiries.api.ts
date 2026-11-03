import { http, type HttpClient } from '@/apis/http';

import { ENQUIRY_PATHS } from './const';
import type { EnquiriesApi } from './interface';

export function createEnquiriesApi(client: HttpClient): EnquiriesApi {
  return {
    create: (body) => client.post(ENQUIRY_PATHS.create, body),
    inbound: (body) => client.post(ENQUIRY_PATHS.inbound, body),
    counts: (signal) => client.get(ENQUIRY_PATHS.counts, { signal }),
    list: (params, signal) =>
      client.get(ENQUIRY_PATHS.list, { signal, query: { lane: params.lane, status: params.status, q: params.q, limit: params.limit } }),
    get: (ref, signal) => client.get(ENQUIRY_PATHS.detail(ref), { signal }),
    route: (ref, body) => client.post(ENQUIRY_PATHS.route(ref), body),
    correct: (ref, body) => client.patch(ENQUIRY_PATHS.fields(ref), body),
    editDraft: (ref, body) => client.put(ENQUIRY_PATHS.draft(ref), { body }),
    send: (ref) => client.post(ENQUIRY_PATHS.send(ref)),
    setStatus: (ref, status) => client.post(ENQUIRY_PATHS.status(ref), { status }),
    retryAi: (ref) => client.post(ENQUIRY_PATHS.retryAi(ref)),
  };
}

export const enquiriesApi = createEnquiriesApi(http);
