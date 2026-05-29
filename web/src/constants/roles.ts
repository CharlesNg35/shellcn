// Role identifiers — must mirror models.Role on the backend. Never hardcode the
// raw strings elsewhere; reference these constants.
export const Role = {
  Viewer: "viewer",
  Operator: "operator",
  Admin: "admin",
} as const;

export type Role = (typeof Role)[keyof typeof Role];

export interface RoleOption {
  value: Role;
  label: string;
  description: string;
}

export const ROLE_OPTIONS: RoleOption[] = [
  {
    value: Role.Viewer,
    label: "Viewer",
    description: "Uses only connections and credentials shared with them.",
  },
  {
    value: Role.Operator,
    label: "Operator",
    description: "Creates and manages their own connections and credentials.",
  },
  {
    value: Role.Admin,
    label: "Admin",
    description: "Manages user accounts; no access to others' resources.",
  },
];
