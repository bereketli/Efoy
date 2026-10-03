"use client";

import { useState } from "react";
import {
  Bell,
  Briefcase,
  Building2,
  CalendarCheck,
  Car,
  CheckCircle2,
  FileSpreadsheet,
  Gauge,
  HeartHandshake,
  MapPinned,
  QrCode,
  Receipt,
  Route as RouteIcon,
  ShieldCheck,
  Smartphone,
  Wallet,
} from "lucide-react";

import { cn } from "@/lib/utils";

const AUDIENCES = [
  {
    key: "parents",
    tab: "Parents",
    icon: HeartHandshake,
    title: "Know your child is safe, every single trip.",
    text: "Subscribe once for the term, then watch the shuttle on a live map and get an alert the moment your child is picked up and dropped off.",
    points: [
      [MapPinned, "Live map with ETAs to your stop"],
      [Bell, "Pickup and drop-off alerts by push or SMS"],
      [ShieldCheck, "Authorised pickup people with photo"],
      [Wallet, "Pay with Telebirr, Chapa or bank transfer"],
    ],
    metric: ["2 alerts", "every school day, per child"],
  },
  {
    key: "riders",
    tab: "Civil servants",
    icon: Briefcase,
    title: "A reserved seat to the office at a price you can plan.",
    text: "No more queues at the minibus stop or surge prices at rush hour. Your seat is held for the whole month, morning and evening.",
    points: [
      [CalendarCheck, "Fixed entry and exit times on working days"],
      [QrCode, "Board with a QR code in seconds"],
      [Receipt, "Monthly receipt with VAT for your records"],
      [Smartphone, "Amharic, English and Afaan Oromoo"],
    ],
    metric: ["Up to 16", "colleagues share one vehicle's cost"],
  },
  {
    key: "institutions",
    tab: "Institutions",
    icon: Building2,
    title: "Transport for your whole school or office, without a fleet.",
    text: "Import your roster from a spreadsheet, set your timetable and holidays, and choose who pays: families, the institution, or a split.",
    points: [
      [FileSpreadsheet, "CSV roster import with guardian invites"],
      [CalendarCheck, "Timetables, early exits and closures"],
      [CheckCircle2, "Approve subscriptions you sponsor"],
      [Receipt, "One monthly invoice with VAT"],
    ],
    metric: ["3 billing modes", "rider pays, institution pays or split"],
  },
  {
    key: "drivers",
    tab: "Drivers",
    icon: Car,
    title: "A guaranteed daily route instead of idle hours.",
    text: "Taxi and Yango drivers get a contracted morning and evening run with a fixed payout per trip, plus standby fees on call days.",
    points: [
      [RouteIcon, "The same route, stops and riders every day"],
      [ShieldCheck, "Simple check-in with a safety checklist"],
      [Wallet, "Weekly payouts, net of commission"],
      [Gauge, "Fair ratings and clear penalties"],
    ],
    metric: ["Fixed pay", "per trip, known in advance"],
  },
  {
    key: "ops",
    tab: "Operations",
    icon: Gauge,
    title: "One console to keep every trip running.",
    text: "Dispatchers see the day's trips, gaps and incidents at a glance; finance reviews transfers and invoices against a double-entry ledger.",
    points: [
      [MapPinned, "Route planning on real roads"],
      [ShieldCheck, "Driver and vehicle document review"],
      [Receipt, "Payments, invoices and a balanced ledger"],
      [CheckCircle2, "Every admin action in the audit log"],
    ],
    metric: ["2-factor", "sign-in for every staff account"],
  },
] as const;

export function Audiences() {
  const [active, setActive] = useState(0);
  const a = AUDIENCES[active];

  return (
    <div>
      <div className="no-scrollbar -mx-4 flex gap-2 overflow-x-auto px-4 pb-1 sm:justify-center">
        {AUDIENCES.map((x, i) => (
          <button
            key={x.key}
            type="button"
            onClick={() => setActive(i)}
            className={cn(
              "flex shrink-0 items-center gap-2 rounded-full px-4 py-2 text-sm font-medium transition-all",
              i === active ? "bg-slate-900 text-white shadow-lg shadow-slate-900/20" : "bg-white text-slate-600 ring-1 ring-slate-200 hover:text-slate-900",
            )}
          >
            <x.icon className="size-4" />
            {x.tab}
          </button>
        ))}
      </div>

      <div key={a.key} className="tab-in mt-10 grid items-center gap-10 lg:grid-cols-2">
        <div>
          <h3 className="text-3xl font-semibold tracking-tight text-balance text-slate-900 sm:text-4xl">{a.title}</h3>
          <p className="mt-4 text-lg leading-relaxed text-slate-600">{a.text}</p>
          <ul className="mt-8 grid gap-3 sm:grid-cols-2">
            {a.points.map(([Icon, label], i) => (
              <li key={label} className="tab-item flex items-center gap-3 rounded-2xl bg-white p-3 ring-1 ring-slate-200" style={{ animationDelay: `${120 + i * 70}ms` }}>
                <span className="grid size-9 shrink-0 place-items-center rounded-xl bg-teal-50 text-teal-700">
                  <Icon className="size-4.5" />
                </span>
                <span className="text-sm font-medium text-slate-700">{label}</span>
              </li>
            ))}
          </ul>
        </div>
        <AudienceVisual index={active} metric={a.metric} />
      </div>
    </div>
  );
}

function AudienceVisual({ index, metric }: { index: number; metric: readonly [string, string] }) {
  const Icon = AUDIENCES[index].icon;
  return (
    <div className="relative">
      <div className="absolute -inset-6 rounded-[2.5rem] bg-gradient-to-br from-teal-200/50 via-cyan-100/40 to-amber-100/50 blur-2xl" />
      <div className="relative overflow-hidden rounded-3xl bg-[#07111f] p-6 text-white shadow-2xl sm:p-8">
        <div className="grid-bg absolute inset-0 opacity-30" />
        <div className="relative">
          <div className="flex items-center justify-between">
            <span className="grid size-12 place-items-center rounded-2xl bg-teal-400 text-[#07111f]">
              <Icon className="size-6" />
            </span>
            <span className="rounded-full bg-white/10 px-3 py-1 text-xs text-slate-300">efoy · {AUDIENCES[index].tab.toLowerCase()}</span>
          </div>
          <div className="mt-8 text-5xl font-semibold tracking-tight">{metric[0]}</div>
          <div className="mt-1 text-slate-400">{metric[1]}</div>
          <div className="mt-8 space-y-2.5">
            {VISUAL_ROWS[index].map(([label, value, tone], i) => (
              <div key={label} className="tab-item flex items-center justify-between rounded-xl bg-white/5 px-4 py-3 ring-1 ring-white/10" style={{ animationDelay: `${200 + i * 90}ms` }}>
                <span className="text-sm text-slate-300">{label}</span>
                <span className={cn("rounded-full px-2.5 py-0.5 text-xs font-medium", TONES[tone])}>{value}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

const TONES = {
  ok: "bg-emerald-400/15 text-emerald-300",
  info: "bg-sky-400/15 text-sky-300",
  warn: "bg-amber-400/15 text-amber-300",
} as const;

const VISUAL_ROWS: [string, string, keyof typeof TONES][][] = [
  [
    ["Picked up · Gerji", "07:12", "ok"],
    ["Arriving at school", "4 min", "info"],
    ["Term subscription", "Active", "ok"],
  ],
  [
    ["Morning seat · MUS-01", "08:15 arrival", "info"],
    ["Evening seat", "17:40 departure", "info"],
    ["October", "Paid · Telebirr", "ok"],
  ],
  [
    ["Roster import", "212 riders", "ok"],
    ["Waiting for approval", "6", "warn"],
    ["Invoice EFY-2026-000002", "Issued", "info"],
  ],
  [
    ["Today · BSS-01", "Checked in", "ok"],
    ["Next run", "15:40", "info"],
    ["This week", "10 trips", "ok"],
  ],
  [
    ["Trips today", "13 · 4 completed", "ok"],
    ["Documents to review", "5", "warn"],
    ["Ledger", "Balanced", "ok"],
  ],
];
