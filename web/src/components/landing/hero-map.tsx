"use client";

import { useEffect, useRef, useState } from "react";
import type { ExpressionSpecification, GeoJSONSource, Map as MapLibreMap, Marker } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";

import { cn } from "@/lib/utils";

import { ROUTES, ROUTE_COLOURS, ZONES, bounds, darkStyle, easeInOut, measure, pointAt, prefersReducedMotion } from "./geo";

const DRAW_MS = 2600;
const measured = ROUTES.map((r) => measure(r.path));

export function drawGradient(colour: string, progress: number): ExpressionSpecification {
  if (progress >= 1) return ["interpolate", ["linear"], ["line-progress"], 0, colour, 1, colour];
  return ["step", ["line-progress"], colour, Math.max(progress, 0.0001), "rgba(0,0,0,0)"];
}

// HeroMap is the living backdrop of the landing page: the city's zones, real
// OSRM-planned routes drawing themselves in, and shuttles driving them.
export function HeroMap({ className }: { className?: string }) {
  const container = useRef<HTMLDivElement>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    let map: MapLibreMap | undefined;
    let frame = 0;
    let visible = true;
    const markers: Marker[] = [];
    const still = prefersReducedMotion();

    const observer = new IntersectionObserver(([e]) => (visible = e.isIntersecting));
    if (container.current) observer.observe(container.current);

    import("maplibre-gl").then(({ default: maplibregl }) => {
      if (cancelled || !container.current) return;
      const all = ROUTES.flatMap((r) => r.path);
      map = new maplibregl.Map({
        container: container.current,
        style: darkStyle,
        bounds: bounds(all),
        fitBoundsOptions: { padding: { top: 90, bottom: 70, left: 60, right: 60 } },
        pitch: 42,
        bearing: -12,
        interactive: false,
        attributionControl: { compact: true },
        fadeDuration: 0,
      });
      const m = map;

      m.on("load", () => {
        if (cancelled) return;
        m.addSource("zones", {
          type: "geojson",
          data: { type: "FeatureCollection", features: ZONES.map((z) => ({ type: "Feature", properties: { name: z.name }, geometry: z.geometry })) },
        });
        m.addLayer({ id: "zones-fill", type: "fill", source: "zones", paint: { "fill-color": "#2dd4bf", "fill-opacity": 0.035 } });
        m.addLayer({
          id: "zones-line",
          type: "line",
          source: "zones",
          paint: { "line-color": "#5eead4", "line-opacity": 0.22, "line-width": 1, "line-dasharray": [2, 3] },
        });

        ROUTES.forEach((r, i) => {
          const colour = ROUTE_COLOURS[i % ROUTE_COLOURS.length];
          m.addSource(`route-${i}`, {
            type: "geojson",
            lineMetrics: true,
            data: { type: "Feature", properties: {}, geometry: { type: "LineString", coordinates: r.path } },
          });
          m.addLayer({
            id: `route-glow-${i}`,
            type: "line",
            source: `route-${i}`,
            layout: { "line-cap": "round", "line-join": "round" },
            paint: { "line-width": 10, "line-blur": 8, "line-opacity": 0.45, "line-gradient": drawGradient(colour, still ? 1 : 0) },
          });
          m.addLayer({
            id: `route-${i}`,
            type: "line",
            source: `route-${i}`,
            layout: { "line-cap": "round", "line-join": "round" },
            paint: { "line-width": 3, "line-gradient": drawGradient(colour, still ? 1 : 0) },
          });
        });

        m.addSource("stops", {
          type: "geojson",
          data: {
            type: "FeatureCollection",
            features: ROUTES.flatMap((r, i) =>
              r.stops.slice(0, -1).map((s) => ({
                type: "Feature" as const,
                properties: { colour: ROUTE_COLOURS[i % ROUTE_COLOURS.length] },
                geometry: { type: "Point" as const, coordinates: s.at },
              })),
            ),
          },
        });
        m.addLayer({
          id: "stops",
          type: "circle",
          source: "stops",
          paint: {
            "circle-radius": 3.2,
            "circle-color": "#07111f",
            "circle-stroke-width": 1.6,
            "circle-stroke-color": ["get", "colour"],
            "circle-opacity": still ? 1 : 0,
            "circle-stroke-opacity": still ? 1 : 0,
          },
        });

        m.addSource("vehicles", { type: "geojson", data: { type: "FeatureCollection", features: [] } });
        m.addLayer({
          id: "vehicles-halo",
          type: "circle",
          source: "vehicles",
          paint: { "circle-radius": 16, "circle-color": ["get", "colour"], "circle-opacity": 0.22, "circle-blur": 0.8 },
        });
        m.addLayer({
          id: "vehicles",
          type: "circle",
          source: "vehicles",
          paint: { "circle-radius": 5.5, "circle-color": "#ffffff", "circle-stroke-width": 3, "circle-stroke-color": ["get", "colour"] },
        });

        // Institutions: the final stop of each route, with a pulsing beacon.
        const seen = new Set<string>();
        ROUTES.forEach((r, i) => {
          const gate = r.stops[r.stops.length - 1];
          if (seen.has(gate.name)) return;
          seen.add(gate.name);
          const el = document.createElement("div");
          el.className = "beacon";
          el.style.setProperty("--beacon", ROUTE_COLOURS[i % ROUTE_COLOURS.length]);
          markers.push(new maplibregl.Marker({ element: el }).setLngLat(gate.at).addTo(m));
        });

        setLoaded(true);
        const start = performance.now();
        let lastData = 0;
        const tick = (now: number) => {
          frame = requestAnimationFrame(tick);
          if (!visible || document.hidden) return;
          const t = now - start;

          if (t < DRAW_MS + ROUTES.length * 180 + 100) {
            ROUTES.forEach((_, i) => {
              const p = easeInOut(Math.min(Math.max((t - i * 180) / DRAW_MS, 0), 1));
              const g = drawGradient(ROUTE_COLOURS[i % ROUTE_COLOURS.length], p);
              m.setPaintProperty(`route-${i}`, "line-gradient", g);
              m.setPaintProperty(`route-glow-${i}`, "line-gradient", g);
            });
            const s = Math.min(Math.max((t - DRAW_MS * 0.6) / 900, 0), 1);
            m.setPaintProperty("stops", "circle-opacity", s);
            m.setPaintProperty("stops", "circle-stroke-opacity", s);
          }

          // Shuttles appear once their route is drawn, then loop at a pace
          // proportional to the planned duration.
          if (now - lastData > 33 && t > DRAW_MS * 0.8) {
            lastData = now;
            const features = ROUTES.flatMap((r, i) => {
              const period = r.min * 900;
              const runs = [((t - DRAW_MS * 0.8) / period + i * 0.17) % 1];
              if (r.km > 15) runs.push((runs[0] + 0.5) % 1);
              return runs.map((f) => ({
                type: "Feature" as const,
                properties: { colour: ROUTE_COLOURS[i % ROUTE_COLOURS.length] },
                geometry: { type: "Point" as const, coordinates: pointAt(measured[i], f).at },
              }));
            });
            (m.getSource("vehicles") as GeoJSONSource).setData({ type: "FeatureCollection", features });
          }
          m.setBearing(-12 + Math.sin(t / 16000) * 7);
        };
        if (!still) frame = requestAnimationFrame(tick);
      });
    });

    return () => {
      cancelled = true;
      cancelAnimationFrame(frame);
      observer.disconnect();
      markers.forEach((mk) => mk.remove());
      map?.remove();
    };
  }, []);

  return (
    <div className={cn("absolute inset-0", className)}>
      <div ref={container} className={cn("size-full transition-opacity duration-1000", loaded ? "opacity-100" : "opacity-0")} />
    </div>
  );
}
