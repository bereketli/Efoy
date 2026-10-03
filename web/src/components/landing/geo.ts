import type { StyleSpecification } from "maplibre-gl";

import data from "./addis.json";

// Real Addis Ababa routes planned by OSRM, exported from the demo database.
export type LngLat = [number, number];
export type Stop = { name: string; at: LngLat; offset: number };
export type Route = { code: string; name: string; km: number; min: number; path: LngLat[]; stops: Stop[] };
export type Zone = { name: string; geometry: GeoJSON.Polygon | GeoJSON.MultiPolygon };

export const ROUTES = data.routes as Route[];
export const ZONES = data.zones as Zone[];

export const ROUTE_COLOURS = ["#2dd4bf", "#fbbf24", "#a78bfa", "#38bdf8", "#fb7185", "#4ade80"];

// A dark basemap made from the OpenStreetMap raster tiles: inverting the
// brightness and turning the hue back keeps roads light on a navy ground.
export const darkStyle: StyleSpecification = {
  version: 8,
  sources: {
    osm: {
      type: "raster",
      tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
      tileSize: 256,
      maxzoom: 19,
      attribution: "© OpenStreetMap contributors",
    },
  },
  layers: [
    { id: "ground", type: "background", paint: { "background-color": "#07111f" } },
    {
      id: "osm",
      type: "raster",
      source: "osm",
      paint: {
        "raster-brightness-min": 1,
        "raster-brightness-max": 0,
        "raster-hue-rotate": 200,
        "raster-saturation": -0.7,
        "raster-contrast": 0.15,
        "raster-opacity": 0.62,
      },
    },
  ],
};

// Measure gives cumulative distances along a path so points can be placed by
// fraction travelled.
export function measure(path: LngLat[]) {
  const cum = [0];
  for (let i = 1; i < path.length; i++) {
    const [x1, y1] = path[i - 1];
    const [x2, y2] = path[i];
    const dx = (x2 - x1) * Math.cos((y1 * Math.PI) / 180);
    cum.push(cum[i - 1] + Math.hypot(dx, y2 - y1));
  }
  return { path, cum, total: cum[cum.length - 1] };
}

export type Measured = ReturnType<typeof measure>;

export function pointAt(m: Measured, fraction: number): { at: LngLat; bearing: number } {
  const target = Math.min(Math.max(fraction, 0), 1) * m.total;
  let lo = 0;
  let hi = m.cum.length - 1;
  while (lo < hi - 1) {
    const mid = (lo + hi) >> 1;
    if (m.cum[mid] <= target) lo = mid;
    else hi = mid;
  }
  const seg = m.cum[hi] - m.cum[lo] || 1;
  const t = (target - m.cum[lo]) / seg;
  const [x1, y1] = m.path[lo];
  const [x2, y2] = m.path[hi];
  const bearing = (Math.atan2((x2 - x1) * Math.cos((y1 * Math.PI) / 180), y2 - y1) * 180) / Math.PI;
  return { at: [x1 + (x2 - x1) * t, y1 + (y2 - y1) * t], bearing };
}

// Fraction of the path nearest to a stop, so stops can be matched to progress.
export function fractionOf(m: Measured, at: LngLat) {
  let best = 0;
  let bestD = Infinity;
  m.path.forEach(([x, y], i) => {
    const d = Math.hypot(x - at[0], y - at[1]);
    if (d < bestD) {
      bestD = d;
      best = i;
    }
  });
  return m.cum[best] / m.total;
}

export function bounds(points: LngLat[]): [LngLat, LngLat] {
  const xs = points.map((p) => p[0]);
  const ys = points.map((p) => p[1]);
  return [
    [Math.min(...xs), Math.min(...ys)],
    [Math.max(...xs), Math.max(...ys)],
  ];
}

export const easeInOut = (t: number) => (t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2);

export function prefersReducedMotion() {
  return typeof window !== "undefined" && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
