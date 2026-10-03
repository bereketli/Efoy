"use client";

import { useEffect, useRef, useState } from "react";
import { AlertTriangle, BellRing, CheckCircle2, Radar, Repeat2 } from "lucide-react";

import { cn } from "@/lib/utils";

import { prefersReducedMotion } from "./geo";
import { useInView } from "./motion";

const LOOP_MS = 14000;
const BREAK_AT = 0.42; // where on the road the shuttle breaks down

const STEPS = [
  { at: 0, until: 0.22, icon: Radar, title: "Shuttle on its route", text: "GPS every 3–5 seconds. Guardians see it move." },
  { at: 0.22, until: 0.34, icon: AlertTriangle, title: "Incident detected", text: "Driver reports a fault, or the trip stalls; dispatch is alerted at once." },
  { at: 0.34, until: 0.62, icon: BellRing, title: "Standby dispatched", text: "The nearest vetted standby vehicle in the zone accepts and heads over." },
  { at: 0.62, until: 0.74, icon: Repeat2, title: "Riders hand over", text: "Every rider is scanned onto the replacement. Tracking switches automatically." },
  { at: 0.74, until: 1.01, icon: CheckCircle2, title: "Delivered", text: "Guardians are told at every step. Replacement on site within 30 minutes, or the trip is credited." },
];

type Pt = { x: number; y: number };

function pointOn(p: SVGPathElement | null, f: number): Pt {
  if (!p) return { x: -100, y: -100 };
  const pt = p.getPointAtLength(p.getTotalLength() * Math.min(Math.max(f, 0), 1));
  return { x: pt.x, y: pt.y };
}

const lerp = (a: number, b: number, t: number) => a + (b - a) * Math.min(Math.max(t, 0), 1);
const ease = (t: number) => 1 - (1 - Math.min(Math.max(t, 0), 1)) ** 3;

export function Continuity() {
  const road = useRef<SVGPathElement>(null);
  const spur = useRef<SVGPathElement>(null);
  const { ref, inView } = useInView<HTMLDivElement>(0.3);
  const [frame, setFrame] = useState<{ t: number; a: Pt; b: Pt } | null>(null);

  useEffect(() => {
    if (!inView) return;
    const still = prefersReducedMotion();
    let id = 0;
    const start = performance.now();
    const tick = (now: number) => {
      const t = still ? 0.8 : ((now - start) % LOOP_MS) / LOOP_MS;
      // Shuttle A drives to the breakdown point and stays there; standby B
      // leaves the depot, reaches A, then finishes the route.
      const a = pointOn(road.current, lerp(0, BREAK_AT, ease(t / 0.22)));
      const b = t < 0.62 ? pointOn(spur.current, ease((t - 0.34) / 0.28)) : pointOn(road.current, lerp(BREAK_AT, 1, ease((t - 0.74) / 0.24)));
      setFrame({ t, a, b });
      if (!still) id = requestAnimationFrame(tick);
    };
    id = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(id);
  }, [inView]);

  const t = frame?.t ?? 0;
  const a = frame?.a ?? { x: -100, y: -100 };
  const b = frame?.b ?? { x: -100, y: -100 };
  const broken = t >= 0.22;
  const bVisible = frame !== null && t >= 0.34;
  const handover = t >= 0.62 && t < 0.74 ? (t - 0.62) / 0.12 : t >= 0.74 ? 1 : 0;
  const active = STEPS.findIndex((s) => t >= s.at && t < s.until);

  return (
    <div ref={ref} className="grid items-center gap-8 lg:grid-cols-[1.3fr_1fr]">
      <div className="relative overflow-hidden rounded-3xl border border-white/10 bg-[#07111f] p-4 shadow-2xl">
        <div className="grid-bg absolute inset-0 opacity-40" />
        <svg viewBox="0 0 800 340" className="relative w-full">
          <defs>
            <linearGradient id="road" x1="0" x2="1">
              <stop offset="0" stopColor="#2dd4bf" />
              <stop offset="1" stopColor="#38bdf8" />
            </linearGradient>
            <filter id="glow" x="-50%" y="-50%" width="200%" height="200%">
              <feGaussianBlur stdDeviation="6" result="b" />
              <feMerge>
                <feMergeNode in="b" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>
          {/* Zone outline and depot */}
          <rect x="230" y="18" width="200" height="74" rx="14" fill="#fbbf24" fillOpacity="0.06" stroke="#fbbf24" strokeOpacity="0.35" strokeDasharray="4 5" />
          <text x="246" y="42" fill="#fcd34d" fontSize="13" fontWeight="600">Standby pool · Bole zone</text>
          <text x="246" y="62" fill="#94a3b8" fontSize="11">2 vetted vehicles on call 06:00–09:00</text>

          <path
            ref={road}
            d="M 40 250 C 160 250, 210 190, 330 196 S 520 268, 610 236 S 720 170, 760 160"
            fill="none"
            stroke="#1e293b"
            strokeWidth="14"
            strokeLinecap="round"
          />
          <path d="M 40 250 C 160 250, 210 190, 330 196 S 520 268, 610 236 S 720 170, 760 160" fill="none" stroke="url(#road)" strokeWidth="3" strokeDasharray="2 10" strokeLinecap="round" opacity="0.7" />
          <path
            ref={spur}
            d="M 400 82 C 410 130, 450 150, 420 205"
            fill="none"
            stroke="#fbbf24"
            strokeOpacity={bVisible && t < 0.62 ? 0.7 : 0.18}
            strokeWidth="2.5"
            strokeDasharray="6 7"
            className="dash-flow"
          />
          {/* School */}
          <g transform="translate(760 160)">
            <circle r="22" fill="#2dd4bf" fillOpacity="0.12" />
            <circle r="9" fill="#2dd4bf" />
            <text x="-30" y="-30" fill="#e2e8f0" fontSize="13" fontWeight="600">School</text>
          </g>
          <g transform="translate(40 250)">
            <circle r="6" fill="#64748b" />
            <text x="-6" y="28" fill="#94a3b8" fontSize="11">First stop</text>
          </g>

          {/* Shuttle A */}
          <g transform={`translate(${a.x} ${a.y})`} filter="url(#glow)">
            {broken && <circle r="26" fill="#f43f5e" fillOpacity="0.18" className="ping-slow" />}
            <rect x="-17" y="-11" width="34" height="22" rx="7" fill={broken ? "#475569" : "#2dd4bf"} />
            <rect x="-11" y="-6" width="22" height="7" rx="2" fill="#07111f" opacity="0.55" />
            {broken && (
              <g transform="translate(0 -30)">
                <rect x="-62" y="-14" width="124" height="24" rx="12" fill="#f43f5e" />
                <text x="0" y="3" textAnchor="middle" fill="white" fontSize="11" fontWeight="600">
                  Engine fault · 07:18
                </text>
              </g>
            )}
          </g>
          {/* Riders moving across */}
          {handover > 0 &&
            handover < 1 &&
            [0, 1, 2, 3].map((i) => {
              const f = Math.min(Math.max(handover * 1.6 - i * 0.2, 0), 1);
              return <circle key={i} cx={lerp(a.x, b.x, f)} cy={lerp(a.y, b.y, f) - 14 * Math.sin(f * Math.PI)} r="4" fill="#e2e8f0" />;
            })}
          {/* Standby B */}
          {bVisible && (
            <g transform={`translate(${b.x} ${b.y})`} filter="url(#glow)">
              <rect x="-17" y="-11" width="34" height="22" rx="7" fill="#fbbf24" />
              <rect x="-11" y="-6" width="22" height="7" rx="2" fill="#07111f" opacity="0.55" />
              {t >= 0.34 && t < 0.62 && (
                <g transform="translate(0 -28)">
                  <rect x="-48" y="-13" width="96" height="22" rx="11" fill="#0f172a" stroke="#fbbf24" strokeOpacity="0.6" />
                  <text x="0" y="3" textAnchor="middle" fill="#fcd34d" fontSize="11" fontWeight="600">
                    ETA {Math.max(1, Math.round(14 * (1 - (t - 0.34) / 0.28)))} min
                  </text>
                </g>
              )}
            </g>
          )}
        </svg>
      </div>

      <ol className="relative space-y-3">
        {STEPS.map((s, i) => (
          <li
            key={s.title}
            className={cn(
              "flex gap-4 rounded-2xl border p-4 transition-all duration-500",
              i === active ? "border-teal-400/40 bg-teal-400/10 shadow-lg shadow-teal-950/30" : "border-white/5 bg-white/[0.02] opacity-60",
            )}
          >
            <span className={cn("grid size-10 shrink-0 place-items-center rounded-xl", i === active ? "bg-teal-400 text-[#07111f]" : "bg-white/5 text-slate-400")}>
              <s.icon className="size-5" />
            </span>
            <div>
              <div className="font-semibold text-white">{s.title}</div>
              <div className="mt-0.5 text-sm text-slate-400">{s.text}</div>
            </div>
          </li>
        ))}
      </ol>
    </div>
  );
}
