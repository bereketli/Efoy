// Roles and console access, shared by server and client code.

export const ROLES = [
  "STUDENT_RIDER",
  "CIVIL_SERVANT_RIDER",
  "GUARDIAN",
  "DRIVER",
  "FLEET_OWNER",
  "INSTITUTION_ADMIN",
  "DISPATCHER",
  "SUPPORT_AGENT",
  "SUPER_ADMIN",
] as const;

export type Role = (typeof ROLES)[number];

// Roles that may sign in to the web console and portals (design doc 2.1).
// Riders, guardians and drivers use the mobile apps instead.
export const CONSOLE_ROLES: readonly Role[] = [
  "SUPER_ADMIN",
  "DISPATCHER",
  "SUPPORT_AGENT",
  "INSTITUTION_ADMIN",
  "FLEET_OWNER",
];

export const ROLE_LABELS: Record<Role, string> = {
  STUDENT_RIDER: "Student rider",
  CIVIL_SERVANT_RIDER: "Civil servant rider",
  GUARDIAN: "Guardian",
  DRIVER: "Driver",
  FLEET_OWNER: "Fleet owner",
  INSTITUTION_ADMIN: "Institution admin",
  DISPATCHER: "Dispatcher",
  SUPPORT_AGENT: "Support agent",
  SUPER_ADMIN: "Super admin",
};

export function hasAnyRole(roles: readonly string[], allowed: readonly Role[]): boolean {
  return roles.some((r) => (allowed as readonly string[]).includes(r));
}

export function hasConsoleAccess(roles: readonly string[]): boolean {
  return hasAnyRole(roles, CONSOLE_ROLES);
}
