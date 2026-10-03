"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type { GeoJSONSource, Map as MapLibreMap } from "maplibre-gl";
import { Bell, Check, MapPin, Navigation, ShieldCheck, Users } from "lucide-react";
import "maplibre-gl/dist/maplibre-gl.css";

import { cn } from "@/lib/utils";

import { ROUTES, bounds, darkStyle, fractionOf, measure, pointAt, prefersReducedMotion } from "./geo";
import { drawGradient } from "./hero-map";
import { useInView } from "./motion";

const ROUTE = ROUTES.find((r) => r.code === "BSS-01") ?? ROUTES[0];
const M = measure(ROUTE.path);
const STOP_AT = ROUTE.stops.map((s) => fractionOf(M, s.at));
const RUN_MS = 34000;
const HOLD_MS = 4000;
const CHILD_STOP = 2;
const BOARDING = [3, 2, 1, 2, 1, 2, 1, 0]; // riders boarding at each stop
const START_MIN = 7 * 60;

const clock = (f: number) => {
  const m = Math.round(START_MIN + f * ROUTE.min);
  return `${String(Math.floor(m / 60)).padStart(2, "0")}:${String(m % 60).padStart(2, "0")}`;
};

export function LiveTrip() {
  const container = useRef<HTMLDivElement>(null);
  const { ref: section, inView } = useInView<HTMLDivElement>(0.25);
  const [fraction, setFraction] = useState(0);

  useEffect(() => {
    if (!inView) return;
    let cancelled = false;
    let map: MapLibreMap | undefined;
    let frame = 0;
    const still = prefersReducedMotion();

    import("maplibre-gl").then(({ default: maplibregl }) => {
      if (cancelled || !container.current) return;
      map = new maplibregl.Map({
        container: container.current,
        style: darkStyle,
        bounds: bounds(ROUTE.path),
        fitBoundsOptions: { padding: 48 },
        interactive: false,
        attributionControl: { compact: true },
      });
      const m = map;
      m.on("load", () => {
        if (cancelled) return;
        // Fit with the tilt applied, clear of the overlays at top and bottom.
        m.fitBounds(bounds(ROUTE.path), { padding: { top: 120, bottom: 100, left: 80, right: 80 }, pitch: 18, duration: 0 });
        m.addSource("route", {
          type: "geojson",
          lineMetrics: true,
          data: { type: "Feature", properties: {}, geometry: { type: "LineString", coordinates: ROUTE.path } },
        });
        m.addLayer({
          id: "route-base",
          type: "line",
          source: "route",
          layout: { "line-cap": "round", "line-join": "round" },
          paint: { "line-color": "#334155", "line-width": 5 },
        });
        m.addLayer({
          id: "route-done",
          type: "line",
          source: "route",
          layout: { "line-cap": "round", "line-join": "round" },
          paint: { "line-width": 5, "line-gradient": drawGradient("#2dd4bf", 0) },
        });
        m.addSource("stops", {
          type: "geojson",
          data: {
            type: "FeatureCollection",
            features: ROUTE.stops.map((s, i) => ({
              type: "Feature",
              properties: { i, name: s.name },
              geometry: { type: "Point", coordinates: s.at },
            })),
          },
        });
        m.addLayer({
          id: "stops",
          type: "circle",
          source: "stops",
          paint: { "circle-radius": 5, "circle-color": "#07111f", "circle-stroke-width": 2.5, "circle-stroke-color": "#64748b" },
        });
        m.addSource("bus", { type: "geojson", data: { type: "FeatureCollection", features: [] } });
        m.addLayer({
          id: "bus-halo",
          type: "circle",
          source: "bus",
          paint: { "circle-radius": 22, "circle-color": "#2dd4bf", "circle-opacity": 0.25, "circle-blur": 0.7 },
        });
        m.addLayer({
          id: "bus",
          type: "circle",
          source: "bus",
          paint: { "circle-radius": 7, "circle-color": "#ffffff", "circle-stroke-width": 4, "circle-stroke-color": "#2dd4bf" },
        });

        const start = performance.now();
        let lastUi = 0;
        const tick = (now: number) => {
          const t = (now - start) % (RUN_MS + HOLD_MS);
          const f = still ? 0.62 : Math.min(t / RUN_MS, 1);
          m.setPaintProperty("route-done", "line-gradient", drawGradient("#2dd4bf", f));
          m.setPaintProperty("stops", "circle-stroke-color", [
            "case",
            ["<=", ["at", ["get", "i"], ["literal", STOP_AT]], f + 0.002],
            "#2dd4bf",
            "#64748b",
          ]);
          (m.getSource("bus") as GeoJSONSource).setData({
            type: "Feature",
            properties: {},
            geometry: { type: "Point", coordinates: pointAt(M, f).at },
          });
          if (now - lastUi > 90) {
            lastUi = now;
            setFraction(f);
          }
          if (!still) frame = requestAnimationFrame(tick);
        };
        frame = requestAnimationFrame(tick);
      });
    });
    return () => {
      cancelled = true;
      cancelAnimationFrame(frame);
      map?.remove();
    };
  }, [inView]);

  const passed = STOP_AT.filter((s) => s <= fraction + 0.002).length;
  const onBoard = BOARDING.slice(0, passed).reduce((a, b) => a + b, 0);
  const arrived = fraction >= 1;
  const childOn = passed > CHILD_STOP;
  const next = ROUTE.stops[Math.min(passed, ROUTE.stops.length - 1)];
  const etaMin = Math.max(0, Math.round((STOP_AT[Math.min(passed, STOP_AT.length - 1)] - fraction) * ROUTE.min));

  const notes = useMemo(() => {
    const list: { icon: "bell" | "check" | "shield"; text: string; time: string }[] = [];
    if (passed >= 1) list.push({ icon: "shield", text: "Abebe checked in · vehicle inspection passed", time: clock(0) });
    if (passed >= CHILD_STOP) list.push({ icon: "bell", text: `Shuttle is 2 min from ${ROUTE.stops[CHILD_STOP].name}`, time: clock(STOP_AT[CHILD_STOP] - 0.06) });
    if (childOn) list.push({ icon: "check", text: `Yonas boarded at ${ROUTE.stops[CHILD_STOP].name}`, time: clock(STOP_AT[CHILD_STOP]) });
    if (arrived) list.push({ icon: "check", text: "Yonas arrived at school, scanned off", time: clock(1) });
    return list.reverse();
  }, [passed, childOn, arrived]);

  return (
    <div ref={section} className="grid items-stretch gap-6 lg:grid-cols-[1fr_340px]">
      <div className="relative min-h-[420px] overflow-hidden rounded-3xl border border-white/10 bg-[#07111f] shadow-2xl shadow-teal-950/40 lg:min-h-[540px]">
        <div className="absolute inset-0">
          <div ref={container} className="size-full" />
        </div>
        <div className="pointer-events-none absolute inset-x-0 top-0 flex flex-wrap items-start justify-between gap-3 p-4">
          <div className="glass rounded-2xl px-4 py-3 text-white">
            <div className="flex items-center gap-2 text-xs font-medium tracking-wide text-teal-300 uppercase">
              <span className="live-dot" /> Live · {ROUTE.code}
            </div>
            <div className="mt-1 text-base font-semibold">{ROUTE.name}</div>
            <div className="text-xs text-slate-400">
              {ROUTE.km} km · {ROUTE.stops.length} stops · planned {ROUTE.min} min
            </div>
          </div>
          <div className="glass flex gap-4 rounded-2xl px-4 py-3 text-white">
            <Hud label="Clock" value={clock(fraction)} />
            <Hud label="On board" value={`${onBoard}/12`} />
            <Hud label="Status" value={arrived ? "Arrived" : "On time"} accent />
          </div>
        </div>
        <div className="pointer-events-none absolute inset-x-0 bottom-0 p-4">
          <div className="glass rounded-2xl p-3">
            <div className="flex items-center justify-between text-[11px] text-slate-400">
              {ROUTE.stops.map((s, i) => (
                <span key={s.name} className={cn("hidden w-0 flex-1 truncate sm:block", i < passed && "text-teal-300")}>
                  {i === 0 || i === ROUTE.stops.length - 1 ? s.name : ""}
                </span>
              ))}
            </div>
            <div className="relative mt-2 h-1.5 rounded-full bg-white/10">
              <div className="absolute inset-y-0 left-0 rounded-full bg-gradient-to-r from-teal-400 to-cyan-300" style={{ width: `${fraction * 100}%` }} />
              {STOP_AT.map((s, i) => (
                <span
                  key={i}
                  className={cn(
                    "absolute top-1/2 size-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-[#07111f] transition-colors",
                    i < passed ? "bg-teal-300" : "bg-slate-600",
                  )}
                  style={{ left: `${s * 100}%` }}
                />
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* The parent's phone */}
      <div className="phone mx-auto w-full max-w-[340px]">
        <div className="phone-screen flex h-full flex-col">
          <div className="flex items-center justify-between px-5 pt-4 text-[11px] font-semibold text-slate-300">
            <span>{clock(fraction)}</span>
            <span className="h-5 w-20 rounded-full bg-black" />
            <span>4G ▮▮▮</span>
          </div>
          <div className="px-5 pt-4">
            <div className="text-xs text-slate-400">Good morning, Selamawit</div>
            <div className="mt-3 rounded-2xl bg-gradient-to-br from-teal-500 to-cyan-600 p-4 text-white shadow-lg shadow-teal-900/40">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-[11px] tracking-wide uppercase opacity-80">Yonas · Grade 4</div>
                  <div className="mt-0.5 text-lg font-semibold">
                    {arrived ? "Safely at school" : childOn ? "On the way to school" : "Waiting at the stop"}
                  </div>
                </div>
                <div className="grid size-11 place-items-center rounded-xl bg-white/20">
                  {arrived ? <Check className="size-6" /> : <Navigation className="size-5" />}
                </div>
              </div>
              <div className="mt-3 flex items-center gap-2 text-xs opacity-90">
                <MapPin className="size-3.5" />
                {arrived ? "Bole Medhanealem gate" : `Next: ${next.name}`}
                {!arrived && <span className="ml-auto rounded-full bg-white/20 px-2 py-0.5 font-medium">{etaMin} min</span>}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2 px-5 pt-4 text-[11px] text-slate-400">
            <Users className="size-3.5" /> AA-3-B60111 · Toyota HiAce · Abebe K.
          </div>
          <div className="mt-3 flex-1 space-y-2 overflow-hidden px-4 pb-5">
            {notes.map((n) => (
              <div key={n.text} className="note-in flex items-start gap-3 rounded-xl bg-white/5 p-3 ring-1 ring-white/10">
                <span
                  className={cn(
                    "grid size-7 shrink-0 place-items-center rounded-lg",
                    n.icon === "check" ? "bg-emerald-400/15 text-emerald-300" : n.icon === "shield" ? "bg-violet-400/15 text-violet-300" : "bg-amber-400/15 text-amber-300",
                  )}
                >
                  {n.icon === "check" ? <Check className="size-4" /> : n.icon === "shield" ? <ShieldCheck className="size-4" /> : <Bell className="size-4" />}
                </span>
                <div className="min-w-0">
                  <div className="text-[13px] leading-snug text-slate-100">{n.text}</div>
                  <div className="mt-0.5 text-[11px] text-slate-500">{n.time}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function Hud({ label, value, accent }: { label: string; value: string; accent?: boolean }) {
  return (
    <div>
      <div className="text-[10px] tracking-wider text-slate-400 uppercase">{label}</div>
      <div className={cn("text-sm font-semibold tabular-nums", accent && "text-emerald-300")}>{value}</div>
    </div>
  );
}
