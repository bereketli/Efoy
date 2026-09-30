"use client";

import { useQuery } from "@tanstack/react-query";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api/client";

export function ApiStatus() {
  const { data, error, isPending } = useQuery({
    queryKey: ["healthz"],
    queryFn: async () => {
      const { data, response } = await api.GET("/healthz");
      if (!data) throw new Error(`core-api answered HTTP ${response.status}`);
      return data;
    },
    refetchInterval: 15_000,
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between gap-2">
          Core API
          {isPending ? (
            <Badge variant="secondary">Checking…</Badge>
          ) : data ? (
            <Badge variant="success">Online</Badge>
          ) : (
            <Badge variant="destructive">Offline</Badge>
          )}
        </CardTitle>
        <CardDescription>Liveness of core-api via GET /healthz</CardDescription>
      </CardHeader>
      <CardContent className="text-sm">
        {data && (
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
            <dt className="text-muted-foreground">Service</dt>
            <dd className="font-mono">{data.service}</dd>
            <dt className="text-muted-foreground">Version</dt>
            <dd className="font-mono">{data.version}</dd>
          </dl>
        )}
        {error && (
          <p className="text-muted-foreground">
            {error.message}. Is <code className="font-mono">make dev</code> running?
          </p>
        )}
      </CardContent>
    </Card>
  );
}
