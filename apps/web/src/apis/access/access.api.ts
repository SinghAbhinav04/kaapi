import { http, type HttpClient } from '@/apis/http';

import { ACCESS_PATHS } from './const';
import type { AccessApi } from './interface';

export function createAccessApi(client: HttpClient): AccessApi {
  return {
    overview: (signal) => client.get(ACCESS_PATHS.overview, { signal }),
    createRole: (body) => client.post(ACCESS_PATHS.roles, body),
    updateRole: (id, body) => client.put(ACCESS_PATHS.role(id), body),
    deleteRole: (id) => client.delete(ACCESS_PATHS.role(id)),
    members: (signal) => client.get(ACCESS_PATHS.users, { signal }),
    createMember: (body) => client.post(ACCESS_PATHS.users, body),
    updateMember: (id, body) => client.patch(ACCESS_PATHS.user(id), body),
  };
}

export const accessApi = createAccessApi(http);
