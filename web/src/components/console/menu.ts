import {
  AlertTriangle,
  Building2,
  Bus,
  CalendarClock,
  CreditCard,
  FileCheck2,
  LayoutDashboard,
  MapIcon,
  Route,
  ScrollText,
  UserRound,
  Users,
  Wallet,
  type LucideIcon,
} from "lucide-react";

import { CONSOLE_ROLES, type Role } from "@/lib/auth/roles";

export interface MenuItem {
  label: string;
  icon: LucideIcon;
  roles: readonly Role[];
  href?: string; // items without a page yet are listed but disabled
}

// One menu for the ops console, institution portal and fleet portal; each
// user sees the items their roles allow (design doc 2 and 2.1). Pages check
// the same roles on the server, so hiding an item is not the only guard.
export const MENU: MenuItem[] = [
  { label: "Overview", href: "/", icon: LayoutDashboard, roles: CONSOLE_ROLES },
  { label: "Live map", icon: MapIcon, roles: ["SUPER_ADMIN", "DISPATCHER", "SUPPORT_AGENT"] },
  { label: "Incidents", icon: AlertTriangle, roles: ["SUPER_ADMIN", "DISPATCHER", "SUPPORT_AGENT"] },
  { label: "Trips", icon: CalendarClock, roles: ["SUPER_ADMIN", "DISPATCHER", "SUPPORT_AGENT", "INSTITUTION_ADMIN"] },
  { label: "Document review", icon: FileCheck2, roles: ["SUPER_ADMIN", "DISPATCHER"] },
  { label: "Drivers & vehicles", icon: Bus, roles: ["SUPER_ADMIN", "DISPATCHER", "FLEET_OWNER"] },
  { label: "Institutions", icon: Building2, roles: ["SUPER_ADMIN", "INSTITUTION_ADMIN"] },
  { label: "Routes", icon: Route, roles: ["SUPER_ADMIN", "DISPATCHER"] },
  { label: "Payments", icon: CreditCard, roles: ["SUPER_ADMIN", "SUPPORT_AGENT", "INSTITUTION_ADMIN"] },
  { label: "Payouts", icon: Wallet, roles: ["SUPER_ADMIN", "FLEET_OWNER"] },
  { label: "Users & roles", icon: Users, roles: ["SUPER_ADMIN"] },
  { label: "Audit log", icon: ScrollText, roles: ["SUPER_ADMIN"] },
  { label: "Account", href: "/account", icon: UserRound, roles: CONSOLE_ROLES },
];
