import type { Metadata } from "next";

import { Forbidden } from "@/components/console/forbidden";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { CONSOLE_ROLES, ROLE_LABELS, type Role } from "@/lib/auth/roles";
import { canAccess, fetchMe, requireSession } from "@/lib/auth/session";

export const metadata: Metadata = { title: "Account" };

const LANGUAGES: Record<string, string> = { am: "Amharic", en: "English", om: "Afaan Oromo" };

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd>{value ?? "—"}</dd>
    </>
  );
}

export default async function AccountPage() {
  if (!(await canAccess(CONSOLE_ROLES))) return <Forbidden />;
  const session = await requireSession();
  const result = await fetchMe(session.token);

  return (
    <div className="flex flex-col gap-6 p-6">
      <h1 className="text-2xl font-semibold">Account</h1>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Profile</CardTitle>
            <CardDescription>Contact an administrator to change these details.</CardDescription>
          </CardHeader>
          <CardContent className="text-sm">
            {result.ok ? (
              <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2">
                <Row label="Name" value={result.me.full_name || null} />
                <Row label="Email" value={result.me.email} />
                <Row label="Phone" value={result.me.phone_e164} />
                <Row
                  label="Language"
                  value={LANGUAGES[result.me.preferred_language] ?? result.me.preferred_language}
                />
              </dl>
            ) : (
              <p className="text-muted-foreground">Profile unavailable (HTTP {result.status}).</p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Roles</CardTitle>
            <CardDescription>What you can see and do in the console.</CardDescription>
          </CardHeader>
          <CardContent>
            <ul className="flex flex-col gap-2 text-sm">
              {(result.ok ? result.me.roles : []).map((grant) => (
                <li key={`${grant.role}/${grant.scope}/${grant.scope_id ?? ""}`} className="flex items-center gap-2">
                  <Badge variant="secondary">{ROLE_LABELS[grant.role as Role] ?? grant.role}</Badge>
                  <span className="text-muted-foreground">
                    {grant.scope === "GLOBAL" ? "All of Efoy" : `${grant.scope.toLowerCase()} ${grant.scope_id}`}
                  </span>
                </li>
              ))}
            </ul>
            <p className="text-muted-foreground mt-4 text-xs">
              Session refreshes automatically and ends 12 hours after sign-in.
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
