"use client";

import { useState, type FormEvent } from "react";
import { useMutation } from "@tanstack/react-query";
import { ExternalLink } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api/client";
import { DOC_TYPE_LABELS, formatDate, formatDateTime, type Document } from "@/lib/documents";

type Decision = "APPROVE" | "REJECT";

function FilePreview({ document, loading, error }: { document: Document; loading: boolean; error?: string }) {
  if (error) {
    return (
      <p role="alert" className="text-destructive text-sm">
        {error}
      </p>
    );
  }
  if (loading || !document.view_url) {
    return <div className="bg-muted h-[420px] animate-pulse rounded-md" aria-label="Loading file" />;
  }
  if (document.content_type === "application/pdf") {
    return (
      <iframe
        src={document.view_url}
        title={`${DOC_TYPE_LABELS[document.doc_type]} (PDF)`}
        className="h-[560px] w-full rounded-md border"
      />
    );
  }
  return (
    // Pre-signed storage URL: next/image cannot optimise it and must not cache it.
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={document.view_url}
      alt={`${DOC_TYPE_LABELS[document.doc_type]} uploaded for ${document.owner_label}`}
      className="bg-muted max-h-[560px] w-full rounded-md border object-contain"
    />
  );
}

export function DocumentReviewPanel({
  document,
  loadingFile,
  fileError,
  onReviewed,
}: {
  document: Document;
  loadingFile: boolean;
  fileError?: string;
  onReviewed: (doc: Document) => void;
}) {
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");

  const review = useMutation({
    mutationFn: async (decision: Decision) => {
      const { data, error } = await api.POST("/v1/admin/documents/{id}/review", {
        params: { path: { id: document.id } },
        body: decision === "REJECT" ? { decision, reason: reason.trim() } : { decision },
      });
      if (!data) throw new Error(error?.detail ?? "The review could not be saved.");
      return data;
    },
    onSuccess: onReviewed,
  });

  function onReject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (reason.trim()) review.mutate("REJECT");
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle className="text-lg">{DOC_TYPE_LABELS[document.doc_type]}</CardTitle>
          <Badge variant="secondary">{document.owner_type === "DRIVER" ? "Driver" : "Vehicle"}</Badge>
        </div>
        <CardDescription>
          {document.owner_label || "Unknown owner"} · submitted {formatDateTime(document.created_at)}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-6">
        <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-4">
          <div>
            <dt className="text-muted-foreground">Number</dt>
            <dd className="font-mono">{document.doc_number ?? "—"}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Issued</dt>
            <dd>{formatDate(document.issued_on)}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Expires</dt>
            <dd>{formatDate(document.expires_on)}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">File</dt>
            <dd>
              {document.view_url ? (
                <a
                  href={document.view_url}
                  target="_blank"
                  rel="noreferrer"
                  className="text-primary inline-flex items-center gap-1 underline-offset-4 hover:underline"
                >
                  Open <ExternalLink className="size-3" aria-hidden />
                </a>
              ) : (
                "—"
              )}
            </dd>
          </div>
        </dl>

        <FilePreview document={document} loading={loadingFile} error={fileError} />

        {review.isError && (
          <p role="alert" className="text-destructive text-sm">
            {review.error.message}
          </p>
        )}

        {rejecting ? (
          <form onSubmit={onReject} className="flex flex-col gap-3">
            <Label htmlFor="reason">Reason for rejection</Label>
            <Textarea
              id="reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              maxLength={500}
              placeholder="For example: the photo is blurry and the licence number cannot be read."
              autoFocus
              required
            />
            <p className="text-muted-foreground text-xs">The driver sees this reason and can upload a new file.</p>
            <div className="flex gap-2">
              <Button type="submit" variant="destructive" disabled={!reason.trim() || review.isPending}>
                {review.isPending ? "Rejecting…" : "Reject document"}
              </Button>
              <Button type="button" variant="ghost" onClick={() => setRejecting(false)} disabled={review.isPending}>
                Cancel
              </Button>
            </div>
          </form>
        ) : (
          <div className="flex gap-2">
            <Button onClick={() => review.mutate("APPROVE")} disabled={review.isPending || loadingFile}>
              {review.isPending ? "Approving…" : "Approve"}
            </Button>
            <Button variant="outline" onClick={() => setRejecting(true)} disabled={review.isPending}>
              Reject…
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
