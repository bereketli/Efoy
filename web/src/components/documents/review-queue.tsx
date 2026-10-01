"use client";

import { useState } from "react";
import { useInfiniteQuery, useQuery, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, FileText } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { api } from "@/lib/api/client";
import { DOC_TYPE_LABELS, formatDateTime, type Document } from "@/lib/documents";
import { cn } from "@/lib/utils";

import { DocumentReviewPanel } from "./review-panel";

const PAGE_SIZE = 25;

export function ReviewQueue() {
  const queryClient = useQueryClient();
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const queue = useInfiniteQuery({
    queryKey: ["review-queue"],
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ pageParam }) => {
      const { data, error } = await api.GET("/v1/admin/documents", {
        params: { query: { limit: PAGE_SIZE, cursor: pageParam } },
      });
      if (!data) throw new Error(error?.detail ?? "The review queue could not be loaded.");
      return data;
    },
    getNextPageParam: (page) => page.next_cursor ?? undefined,
  });

  const items = queue.data?.pages.flatMap((p) => p.items) ?? [];
  // Keep the chosen document while it is in the queue, else show the oldest.
  const selected = items.find((d) => d.id === selectedId) ?? items[0];

  const detail = useQuery({
    queryKey: ["review-document", selected?.id],
    enabled: !!selected,
    // The view URL expires after 10 minutes; refresh it well before then.
    staleTime: 5 * 60_000,
    queryFn: async () => {
      const { data, error } = await api.GET("/v1/admin/documents/{id}", {
        params: { path: { id: selected!.id } },
      });
      if (!data) throw new Error(error?.detail ?? "The document could not be loaded.");
      return data;
    },
  });

  function onReviewed(reviewed: Document) {
    const index = items.findIndex((d) => d.id === reviewed.id);
    const next = items[index + 1] ?? items[index - 1];
    setSelectedId(next?.id ?? null);
    void queryClient.invalidateQueries({ queryKey: ["review-queue"] });
  }

  if (queue.isPending) {
    return <p className="text-muted-foreground text-sm">Loading the review queue…</p>;
  }
  if (queue.isError) {
    return (
      <p role="alert" className="text-destructive text-sm">
        {queue.error.message}
      </p>
    );
  }
  if (items.length === 0) {
    return (
      <Card>
        <CardContent className="flex flex-col items-center gap-2 py-12 text-center">
          <CheckCircle2 className="text-success size-10" aria-hidden />
          <p className="font-medium">All caught up</p>
          <p className="text-muted-foreground text-sm">No documents are waiting for review.</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="grid items-start gap-6 lg:grid-cols-[minmax(280px,360px)_1fr]">
      <Card className="gap-0 py-0">
        <CardHeader className="border-b py-4">
          <CardTitle className="text-base">Waiting for review</CardTitle>
          <CardDescription>Oldest first</CardDescription>
        </CardHeader>
        <ul className="divide-y" aria-label="Documents waiting for review">
          {items.map((doc) => (
            <li key={doc.id}>
              <button
                type="button"
                onClick={() => setSelectedId(doc.id)}
                aria-current={doc.id === selected?.id ? "true" : undefined}
                className={cn(
                  "flex w-full items-start gap-3 px-4 py-3 text-left transition-colors",
                  doc.id === selected?.id ? "bg-accent" : "hover:bg-accent/50",
                )}
              >
                <FileText className="text-muted-foreground mt-0.5 size-4 shrink-0" aria-hidden />
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="truncate text-sm font-medium">{DOC_TYPE_LABELS[doc.doc_type]}</span>
                  <span className="text-muted-foreground truncate text-xs">
                    {doc.owner_label || "Unknown owner"} · {formatDateTime(doc.created_at)}
                  </span>
                </span>
                <Badge variant="outline" className="text-[10px]">
                  {doc.owner_type === "DRIVER" ? "Driver" : "Vehicle"}
                </Badge>
              </button>
            </li>
          ))}
        </ul>
        {queue.hasNextPage && (
          <div className="border-t p-3">
            <Button
              variant="ghost"
              size="sm"
              className="w-full"
              onClick={() => queue.fetchNextPage()}
              disabled={queue.isFetchingNextPage}
            >
              {queue.isFetchingNextPage ? "Loading…" : "Load more"}
            </Button>
          </div>
        )}
      </Card>

      {selected && (
        <DocumentReviewPanel
          key={selected.id}
          document={detail.data ?? selected}
          loadingFile={detail.isPending}
          fileError={detail.error?.message}
          onReviewed={onReviewed}
        />
      )}
    </div>
  );
}
