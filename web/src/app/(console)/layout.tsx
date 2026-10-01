import { redirect } from "next/navigation";

import { Forbidden } from "@/components/console/forbidden";
import { SidebarNav } from "@/components/console/sidebar-nav";
import { UserMenu } from "@/components/console/user-menu";
import { hasConsoleAccess, ROLE_LABELS, type Role } from "@/lib/auth/roles";
import { fetchMe, requireSession } from "@/lib/auth/session";

// Every page in (console) requires a signed-in user with a console role.
export default async function ConsoleLayout({ children }: { children: React.ReactNode }) {
  const session = await requireSession();
  const me = await fetchMe(session.token);
  if (!me.ok && me.status === 401) redirect("/login");

  const name = me.ok ? me.me.full_name || me.me.email || "Signed in" : "Signed in";
  const roleLabels = session.roles.map((r) => ROLE_LABELS[r as Role] ?? r).join(", ");

  return (
    <div className="flex min-h-screen">
      <aside className="bg-muted/40 hidden w-60 shrink-0 border-r md:block">
        <div className="flex h-14 items-center gap-2 border-b px-5">
          <span className="bg-primary text-primary-foreground grid size-7 place-items-center rounded-md text-sm font-bold">
            E
          </span>
          <span className="font-semibold">Efoy Console</span>
        </div>
        <SidebarNav roles={session.roles} />
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center justify-end border-b px-6">
          <UserMenu name={name} detail={roleLabels} />
        </header>
        <main className="flex-1">
          {!me.ok && (
            <p role="alert" className="bg-destructive/10 text-destructive border-b px-6 py-2 text-sm">
              The Efoy API is not responding. Some information may be missing.
            </p>
          )}
          {hasConsoleAccess(session.roles) ? (
            children
          ) : (
            <Forbidden
              title="This account cannot use the console"
              detail="Riders, guardians and drivers use the Efoy mobile apps."
            />
          )}
        </main>
      </div>
    </div>
  );
}
