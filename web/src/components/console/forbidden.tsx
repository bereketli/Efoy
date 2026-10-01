import { ShieldAlert } from "lucide-react";

export function Forbidden({
  title = "You don't have access to this page",
  detail = "Ask a super admin if you need it.",
}: {
  title?: string;
  detail?: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-24 text-center">
      <ShieldAlert className="text-muted-foreground size-10" aria-hidden />
      <h2 className="text-lg font-semibold">{title}</h2>
      <p className="text-muted-foreground max-w-sm text-sm">{detail}</p>
    </div>
  );
}
