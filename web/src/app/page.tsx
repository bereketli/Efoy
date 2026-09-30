import {
  AlertTriangle,
  Building2,
  Bus,
  CalendarClock,
  CreditCard,
  LayoutDashboard,
  MapIcon,
  Route,
  Users,
} from "lucide-react";

import { ApiStatus } from "@/components/api-status";
import { TestMap } from "@/components/test-map";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

// Console sections from the design doc; each becomes a page as its day lands.
const sections = [
  { label: "Overview", icon: LayoutDashboard, active: true },
  { label: "Live map", icon: MapIcon },
  { label: "Incidents", icon: AlertTriangle },
  { label: "Trips", icon: CalendarClock },
  { label: "Drivers & vehicles", icon: Bus },
  { label: "Institutions", icon: Building2 },
  { label: "Routes", icon: Route },
  { label: "Payments", icon: CreditCard },
  { label: "Users & roles", icon: Users },
];

export default function Home() {
  return (
    <div className="flex min-h-screen">
      <aside className="bg-muted/40 hidden w-60 shrink-0 border-r md:block">
        <div className="flex h-14 items-center gap-2 border-b px-5">
          <span className="bg-primary text-primary-foreground grid size-7 place-items-center rounded-md text-sm font-bold">
            E
          </span>
          <span className="font-semibold">Efoy Console</span>
        </div>
        <nav className="flex flex-col gap-1 p-3 text-sm">
          {sections.map(({ label, icon: Icon, active }) => (
            <span
              key={label}
              aria-current={active ? "page" : undefined}
              aria-disabled={active ? undefined : true}
              className={
                active
                  ? "bg-background flex items-center gap-2 rounded-md px-3 py-2 font-medium shadow-xs"
                  : "text-muted-foreground flex cursor-not-allowed items-center gap-2 rounded-md px-3 py-2"
              }
            >
              <Icon className="size-4" />
              {label}
            </span>
          ))}
        </nav>
      </aside>

      <main className="flex-1">
        <header className="flex h-14 items-center border-b px-6">
          <h1 className="text-lg font-semibold">Overview</h1>
        </header>
        <div className="grid gap-6 p-6 lg:grid-cols-[320px_1fr]">
          <ApiStatus />
          <Card>
            <CardHeader>
              <CardTitle>Addis Ababa</CardTitle>
              <CardDescription>Live vehicles will appear here once tracking is connected.</CardDescription>
            </CardHeader>
            <CardContent>
              <TestMap />
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  );
}
