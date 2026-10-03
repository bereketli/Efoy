"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight, Check, Menu, Minus, Plus, X } from "lucide-react";

import { cn } from "@/lib/utils";

import { ROUTES, ROUTE_COLOURS } from "./geo";

const LINKS = [
  ["How it works", "#how"],
  ["Live tracking", "#live"],
  ["Guarantee", "#guarantee"],
  ["Pricing", "#pricing"],
  ["For you", "#audiences"],
  ["FAQ", "#faq"],
];

export function Logo({ light }: { light?: boolean }) {
  return (
    <span className="flex items-center gap-2.5">
      <span className="relative grid size-9 place-items-center rounded-xl bg-gradient-to-br from-teal-400 to-cyan-500 shadow-lg shadow-teal-500/30">
        {/* A route: from a home stop, along the road, to the gate. */}
        <svg viewBox="0 0 24 24" className="size-5.5 text-[#07111f]" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round">
          <path d="M6 18c5 0 3.5-11 9-11h1.5" />
          <circle cx="6" cy="18" r="2.4" fill="currentColor" stroke="none" />
          <circle cx="18.5" cy="7" r="2.6" />
        </svg>
      </span>
      <span className={cn("text-lg font-semibold tracking-tight", light ? "text-white" : "text-slate-900")}>
        Efoy <span className={cn("font-normal", light ? "text-teal-300" : "text-teal-600")}>እፎይ</span>
      </span>
    </span>
  );
}

export function Nav() {
  const [scrolled, setScrolled] = useState(false);
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 24);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <header className={cn("fixed inset-x-0 top-0 z-50 transition-all duration-500", scrolled ? "py-2" : "py-4")}>
      <div
        className={cn(
          "mx-auto flex max-w-7xl items-center justify-between rounded-2xl px-4 py-2.5 transition-all duration-500 sm:px-5",
          scrolled ? "glass-nav mx-3 shadow-2xl shadow-black/20 sm:mx-6 xl:mx-auto" : "",
        )}
      >
        <a href="#top" aria-label="Efoy home">
          <Logo light />
        </a>
        <nav className="hidden items-center gap-1 lg:flex">
          {LINKS.map(([label, href]) => (
            <a key={href} href={href} className="rounded-lg px-3 py-2 text-sm text-slate-300 transition-colors hover:bg-white/5 hover:text-white">
              {label}
            </a>
          ))}
        </nav>
        <div className="flex items-center gap-2">
          <Link
            href="/login"
            className="group hidden items-center gap-1.5 rounded-xl bg-white px-4 py-2 text-sm font-semibold text-slate-900 shadow-lg shadow-white/10 transition-all hover:bg-teal-50 sm:flex"
          >
            Sign in <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" />
          </Link>
          <button type="button" aria-label="Menu" onClick={() => setOpen((o) => !o)} className="grid size-10 place-items-center rounded-xl text-white hover:bg-white/10 lg:hidden">
            {open ? <X className="size-5" /> : <Menu className="size-5" />}
          </button>
        </div>
      </div>
      {open && (
        <div className="glass tab-in mx-3 mt-2 rounded-2xl p-3 lg:hidden">
          {LINKS.map(([label, href]) => (
            <a key={href} href={href} onClick={() => setOpen(false)} className="block rounded-lg px-3 py-2.5 text-slate-200 hover:bg-white/5">
              {label}
            </a>
          ))}
          <Link href="/login" className="mt-2 flex items-center justify-center gap-2 rounded-xl bg-white px-4 py-2.5 font-semibold text-slate-900">
            Sign in <ArrowRight className="size-4" />
          </Link>
        </div>
      )}
    </header>
  );
}

const EVENTS = [
  ["Yonas boarded", "Gerji Mebrat Hail · 07:12"],
  ["Liya dropped off", "Bole Medhanealem gate · 07:46"],
  ["Standby accepted", "Bole zone · ETA 11 min"],
  ["Payment received", "Telebirr · 2,890 ETB"],
  ["Dagim boarded", "Sidist Kilo · 07:31"],
];

// HeroCards float over the hero map: a cycling trip card and a notification.
export function HeroCards() {
  const [i, setI] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setI((x) => x + 1), 3200);
    return () => clearInterval(id);
  }, []);
  const route = ROUTES[i % ROUTES.length];
  const colour = ROUTE_COLOURS[i % ROUTES.length];
  const event = EVENTS[i % EVENTS.length];

  return (
    <div className="pointer-events-none absolute inset-0 hidden lg:block">
      <div className="float-slow glass absolute top-[24%] right-[6%] w-72 rounded-2xl p-4 text-white shadow-2xl">
        <div key={route.code} className="tab-in">
          <div className="flex items-center justify-between">
            <span className="flex items-center gap-2 text-xs font-semibold tracking-wide uppercase" style={{ color: colour }}>
              <span className="size-2 rounded-full" style={{ background: colour, boxShadow: `0 0 12px ${colour}` }} /> {route.code}
            </span>
            <span className="rounded-full bg-emerald-400/15 px-2 py-0.5 text-[11px] font-medium text-emerald-300">On time</span>
          </div>
          <div className="mt-2 font-semibold">{route.name}</div>
          <div className="mt-3 grid grid-cols-3 gap-2 text-center">
            {[
              [`${route.km}`, "km"],
              [`${route.stops.length}`, "stops"],
              [`${route.min}`, "min"],
            ].map(([v, l]) => (
              <div key={l} className="rounded-lg bg-white/5 py-1.5">
                <div className="text-sm font-semibold tabular-nums">{v}</div>
                <div className="text-[10px] text-slate-400">{l}</div>
              </div>
            ))}
          </div>
        </div>
      </div>
      <div className="float-slower glass absolute right-[22%] bottom-[16%] flex w-72 items-center gap-3 rounded-2xl p-3.5 text-white shadow-2xl">
        <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-emerald-400/15 text-emerald-300">
          <Check className="size-5" />
        </span>
        <div key={event[0]} className="tab-in min-w-0">
          <div className="text-sm font-semibold">{event[0]}</div>
          <div className="truncate text-xs text-slate-400">{event[1]}</div>
        </div>
      </div>
    </div>
  );
}

export function Faq({ items }: { items: [string, string][] }) {
  const [open, setOpen] = useState<number | null>(0);
  return (
    <div className="divide-y divide-slate-200 rounded-3xl border border-slate-200 bg-white">
      {items.map(([q, a], i) => (
        <div key={q}>
          <button
            type="button"
            onClick={() => setOpen(open === i ? null : i)}
            className="flex w-full items-center justify-between gap-6 px-6 py-5 text-left"
            aria-expanded={open === i}
          >
            <span className="font-medium text-slate-900">{q}</span>
            <span className={cn("grid size-8 shrink-0 place-items-center rounded-full transition-colors", open === i ? "bg-teal-500 text-white" : "bg-slate-100 text-slate-500")}>
              {open === i ? <Minus className="size-4" /> : <Plus className="size-4" />}
            </span>
          </button>
          <div className={cn("grid transition-all duration-500 ease-out", open === i ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0")}>
            <div className="overflow-hidden">
              <p className="px-6 pb-6 leading-relaxed text-slate-600">{a}</p>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
