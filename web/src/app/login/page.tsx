import type { Metadata } from "next";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

import { LoginForm } from "./login-form";

export const metadata: Metadata = { title: "Sign in" };

// Only same-site paths are allowed after sign-in, never another origin.
function safeNext(next: string | undefined): string {
  return next && next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/\\") ? next : "/";
}

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ next?: string }> }) {
  const { next } = await searchParams;

  return (
    <main className="bg-muted/40 flex min-h-screen items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="gap-3">
          <span className="bg-primary text-primary-foreground grid size-9 place-items-center rounded-md font-bold">
            E
          </span>
          <CardTitle className="text-xl">Sign in to Efoy</CardTitle>
          <CardDescription>Operations console and institution and fleet portals.</CardDescription>
        </CardHeader>
        <CardContent>
          <LoginForm next={safeNext(next)} />
        </CardContent>
      </Card>
    </main>
  );
}
