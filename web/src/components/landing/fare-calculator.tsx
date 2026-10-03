"use client";

import { useState } from "react";
import { User } from "lucide-react";

import { cn } from "@/lib/utils";

// The fare-sharing model (design doc 5.1) with the illustrative minibus rates.
const BASE = 150;
const PER_KM = 30;
const PER_MIN = 3;
const COMMISSION = 0.15;
const MIN_OCCUPANCY = 10;
const SEATS = 16;
const PLANS = {
  WEEKLY: { label: "Weekly", days: 5, discount: 0 },
  MONTHLY: { label: "Monthly", days: 22, discount: 0.05 },
  TERM: { label: "Term", days: 66, discount: 0.1 },
} as const;

const etb = (n: number) => n.toLocaleString("en-US", { maximumFractionDigits: 0 });

export function FareCalculator() {
  const [km, setKm] = useState(14);
  const [minutes, setMinutes] = useState(45);
  const [riders, setRiders] = useState(12);
  const [shifts, setShifts] = useState<1 | 2>(2);
  const [plan, setPlan] = useState<keyof typeof PLANS>("MONTHLY");

  const payout = BASE + PER_KM * km + PER_MIN * minutes;
  const revenue = payout / (1 - COMMISSION);
  const seat = revenue / Math.max(riders, MIN_OCCUPANCY);
  const p = PLANS[plan];
  const price = Math.ceil((seat * shifts * p.days * (1 - p.discount)) / 10) * 10;
  const solo = Math.ceil((revenue * shifts * p.days) / 10) * 10;

  return (
    <div className="grid gap-6 lg:grid-cols-[1.05fr_1fr]">
      <div className="rounded-3xl border bg-white p-6 shadow-xl shadow-slate-900/5 sm:p-8">
        <div className="space-y-6">
          <Slider label="Route distance" value={km} unit="km" min={4} max={30} onChange={setKm} />
          <Slider label="Planned travel time" value={minutes} unit="min" min={15} max={90} step={5} onChange={setMinutes} />
          <Slider label="Riders sharing the shuttle" value={riders} unit="of 16" min={4} max={16} onChange={setRiders} />
          <div className="grid gap-4 sm:grid-cols-2">
            <Segmented
              label="Trips per day"
              value={String(shifts)}
              options={[
                ["1", "Morning only"],
                ["2", "Both ways"],
              ]}
              onChange={(v) => setShifts(Number(v) as 1 | 2)}
            />
            <Segmented
              label="Plan"
              value={plan}
              options={Object.entries(PLANS).map(([k, v]) => [k, v.label])}
              onChange={(v) => setPlan(v as keyof typeof PLANS)}
            />
          </div>
        </div>
      </div>

      <div className="relative overflow-hidden rounded-3xl bg-[#07111f] p-6 text-white shadow-2xl shadow-teal-950/30 sm:p-8">
        <div className="aurora opacity-60" />
        <div className="relative">
          <div className="text-xs font-medium tracking-wider text-teal-300 uppercase">Your {p.label.toLowerCase()} price</div>
          <div className="mt-2 flex items-baseline gap-2">
            <span key={price} className="price-pop text-5xl font-semibold tracking-tight sm:text-6xl">
              {etb(price)}
            </span>
            <span className="text-lg text-slate-400">ETB</span>
          </div>
          <div className="mt-1 text-sm text-slate-400">
            {etb(seat)} ETB per seat per trip · {p.discount ? `${p.discount * 100}% plan discount` : "no discount"}
          </div>

          <div className="mt-6 grid grid-cols-8 gap-2">
            {Array.from({ length: SEATS }, (_, i) => (
              <div
                key={i}
                className={cn(
                  "grid aspect-square place-items-center rounded-lg transition-all duration-500",
                  i < riders ? "bg-teal-400/90 text-[#07111f] shadow-[0_0_18px_-4px] shadow-teal-300" : "bg-white/5 text-white/20 ring-1 ring-white/10",
                )}
                style={{ transitionDelay: `${i * 18}ms` }}
              >
                <User className="size-4" />
              </div>
            ))}
          </div>
          <div className="mt-2 text-xs text-slate-400">
            {riders < MIN_OCCUPANCY
              ? `Below ${MIN_OCCUPANCY} riders the price is held at the launch rate; Efoy covers the gap.`
              : "Every extra rider makes every seat cheaper."}
          </div>

          <dl className="mt-6 grid grid-cols-3 gap-3 border-t border-white/10 pt-5 text-sm">
            <Fact label="Driver earns / trip" value={`${etb(payout)} ETB`} />
            <Fact label="Seat vs. whole vehicle" value={`${Math.round((price / solo) * 100)}%`} />
            <Fact label="Rounded" value="to 10 ETB" />
          </dl>
        </div>
      </div>
    </div>
  );
}

function Slider(props: { label: string; value: number; unit: string; min: number; max: number; step?: number; onChange: (v: number) => void }) {
  const pct = ((props.value - props.min) / (props.max - props.min)) * 100;
  return (
    <label className="block">
      <div className="flex items-baseline justify-between">
        <span className="text-sm font-medium text-slate-700">{props.label}</span>
        <span className="text-sm text-slate-500 tabular-nums">
          <span className="text-lg font-semibold text-slate-900">{props.value}</span> {props.unit}
        </span>
      </div>
      <input
        type="range"
        className="range mt-3 w-full"
        min={props.min}
        max={props.max}
        step={props.step ?? 1}
        value={props.value}
        style={{ "--pct": `${pct}%` } as React.CSSProperties}
        onChange={(e) => props.onChange(Number(e.target.value))}
      />
    </label>
  );
}

function Segmented(props: { label: string; value: string; options: string[][]; onChange: (v: string) => void }) {
  return (
    <div>
      <div className="text-sm font-medium text-slate-700">{props.label}</div>
      <div className="mt-2 flex rounded-xl bg-slate-100 p-1">
        {props.options.map(([v, label]) => (
          <button
            key={v}
            type="button"
            onClick={() => props.onChange(v)}
            className={cn(
              "flex-1 rounded-lg px-2 py-1.5 text-xs font-medium transition-all sm:text-sm",
              props.value === v ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-800",
            )}
          >
            {label}
          </button>
        ))}
      </div>
    </div>
  );
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-[11px] text-slate-400">{label}</dt>
      <dd className="mt-0.5 font-semibold">{value}</dd>
    </div>
  );
}
