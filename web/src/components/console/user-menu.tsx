"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { LogOut } from "lucide-react";

import { Button } from "@/components/ui/button";

export function UserMenu({ name, detail }: { name: string; detail: string }) {
  const router = useRouter();
  const [pending, setPending] = useState(false);

  async function signOut() {
    setPending(true);
    await fetch("/api/auth/logout", { method: "POST" }).catch(() => undefined);
    router.replace("/login");
    router.refresh();
  }

  return (
    <div className="flex items-center gap-3">
      <div className="text-right leading-tight">
        <div className="text-sm font-medium">{name}</div>
        <div className="text-muted-foreground text-xs">{detail}</div>
      </div>
      <Button variant="outline" size="sm" onClick={signOut} disabled={pending}>
        <LogOut />
        Sign out
      </Button>
    </div>
  );
}
