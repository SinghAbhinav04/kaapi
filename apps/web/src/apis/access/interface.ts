import type {
  AccessResponse,
  CreateMemberRequest,
  MemberResponse,
  MembersResponse,
  RoleRequest,
  RoleResponse,
  UpdateMemberRequest,
} from './types';

/** Team and access: the administrator's people, roles and permissions. */
export interface AccessApi {
  overview(signal?: AbortSignal): Promise<AccessResponse>;
  createRole(body: RoleRequest): Promise<RoleResponse>;
  updateRole(id: string, body: RoleRequest): Promise<RoleResponse>;
  deleteRole(id: string): Promise<void>;
  members(signal?: AbortSignal): Promise<MembersResponse>;
  createMember(body: CreateMemberRequest): Promise<MemberResponse>;
  updateMember(id: string, body: UpdateMemberRequest): Promise<MemberResponse>;
}
