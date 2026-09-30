"use client";

import { useEffect, useRef, useState } from "react";
import type { StyleSpecification } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";

// Meskel Square, Addis Ababa ([lng, lat]).
const ADDIS_ABABA: [number, number] = [38.7613, 9.0107];

// Development fallback: OpenStreetMap raster tiles. Production uses self-hosted
// OSM vector tiles or MapTiler via NEXT_PUBLIC_MAP_STYLE_URL (design doc 8.2).
const osmRasterStyle: StyleSpecification = {
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
  layers: [{ id: "osm", type: "raster", source: "osm" }],
};

export function TestMap() {
  const container = useRef<HTMLDivElement>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let map: import("maplibre-gl").Map | undefined;
    let cancelled = false;

    // maplibre-gl touches `window`, so load it only in the browser.
    import("maplibre-gl")
      .then(({ default: maplibregl }) => {
        if (cancelled || !container.current) return;
        map = new maplibregl.Map({
          container: container.current,
          style: process.env.NEXT_PUBLIC_MAP_STYLE_URL || osmRasterStyle,
          center: ADDIS_ABABA,
          zoom: 12,
        });
        map.addControl(new maplibregl.NavigationControl(), "top-right");
        new maplibregl.Marker({ color: "#0f766e" })
          .setLngLat(ADDIS_ABABA)
          .setPopup(new maplibregl.Popup().setText("Meskel Square"))
          .addTo(map);
      })
      .catch(() => setFailed(true));

    return () => {
      cancelled = true;
      map?.remove();
    };
  }, []);

  if (failed) {
    return <p className="text-muted-foreground text-sm">The map library failed to load.</p>;
  }
  return <div ref={container} className="h-[420px] w-full overflow-hidden rounded-md border" />;
}
