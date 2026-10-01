"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { hasAnyRole } from "@/lib/auth/roles";
import { cn } from "@/lib/utils";

import { MENU } from "./menu";

export function SidebarNav({ roles }: { roles: string[] }) {
  const pathname = usePathname();
  const items = MENU.filter((item) => hasAnyRole(roles, item.roles));

  return (
    <nav aria-label="Main" className="flex flex-col gap-1 p-3 text-sm">
      {items.map(({ label, href, icon: Icon }) => {
        if (!href) {
          return (
            <span
              key={label}
              aria-disabled
              className="text-muted-foreground flex cursor-not-allowed items-center gap-2 rounded-md px-3 py-2"
            >
              <Icon className="size-4" />
              {label}
              <Badge variant="outline" className="ml-auto text-[10px] font-normal">
                Soon
              </Badge>
            </span>
          );
        }
        const active = href === "/" ? pathname === "/" : pathname.startsWith(href);
        return (
          <Link
            key={label}
            href={href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "flex items-center gap-2 rounded-md px-3 py-2 transition-colors",
              active ? "bg-background font-medium shadow-xs" : "hover:bg-background/60",
            )}
          >
            <Icon className="size-4" />
            {label}
          </Link>
        );
      })}
    </nav>
  );
}
