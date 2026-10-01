import type { Metadata } from "next";

import { Forbidden } from "@/components/console/forbidden";
import { ReviewQueue } from "@/components/documents/review-queue";
import type { Role } from "@/lib/auth/roles";
import { canAccess } from "@/lib/auth/session";

export const metadata: Metadata = { title: "Document review" };

// Same roles as core-api's authz.CanReviewDocuments.
const REVIEWERS: readonly Role[] = ["SUPER_ADMIN", "DISPATCHER"];

export default async function DocumentReviewPage() {
  if (!(await canAccess(REVIEWERS))) return <Forbidden />;

  return (
    <div className="flex flex-col gap-6 p-6">
      <div>
        <h1 className="text-2xl font-semibold">Document review</h1>
        <p className="text-muted-foreground text-sm">
          Check each document against the driver or vehicle, then approve it or reject it with a reason the driver
          will see.
        </p>
      </div>
      <ReviewQueue />
    </div>
  );
}
