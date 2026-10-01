import type { components } from "@/lib/api/schema";

export type Document = components["schemas"]["Document"];
export type DocumentType = components["schemas"]["DocumentType"];

export const DOC_TYPE_LABELS: Record<DocumentType, string> = {
  DRIVING_LICENCE: "Driving licence",
  VEHICLE_LIBRE: "Vehicle libre",
  INSURANCE: "Insurance",
  ANNUAL_INSPECTION: "Annual inspection",
  POLICE_CLEARANCE: "Police clearance",
  PHOTO: "Photo",
  YANGO_PROFILE: "Yango profile",
  TRADE_LICENCE: "Trade licence",
  TIN_CERTIFICATE: "TIN certificate",
  OTHER: "Other",
};

const dateFormat = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short", year: "numeric" });
const dateTimeFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
});

// Dates from the API are calendar dates (YYYY-MM-DD); format without shifting time zones.
export function formatDate(date: string | null | undefined): string {
  if (!date) return "—";
  const [y, m, d] = date.split("-").map(Number);
  return dateFormat.format(new Date(Date.UTC(y, m - 1, d)));
}

export function formatDateTime(timestamp: string): string {
  return dateTimeFormat.format(new Date(timestamp));
}
