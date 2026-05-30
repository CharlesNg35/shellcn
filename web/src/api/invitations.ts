import { api } from "./client";
import type { Role } from "../constants/roles";
import type { InvitationSummary, InviteResult } from "../types/projection";

export interface InviteRequest {
  email: string;
  role: Role;
}

export interface AcceptInviteRequest {
  username: string;
  password: string;
}

// Admin-managed invitation lifecycle.
export const invitationsApi = {
  list: () => api.get<InvitationSummary[]>("/admin/invitations"),
  create: (body: InviteRequest) =>
    api.post<InviteResult>("/admin/invitations", body),
  remove: (id: string) => api.del(`/admin/invitations/${id}`),
};

// Public token redemption (no session required).
export const inviteAcceptApi = {
  get: (token: string) => api.get<{ email: string }>(`/invitations/${token}`),
  accept: (token: string, body: AcceptInviteRequest) =>
    api.post(`/invitations/${token}/accept`, body),
};
