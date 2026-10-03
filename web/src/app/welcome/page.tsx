import type { Metadata } from "next";
import Link from "next/link";
import {
  ArrowRight,
  BadgeCheck,
  Banknote,
  Bell,
  Building2,
  CalendarClock,
  Car,
  CreditCard,
  Eye,
  Fingerprint,
  Hourglass,
  Languages,
  LifeBuoy,
  MapPinned,
  MessageSquareText,
  QrCode,
  Radio,
  ScanLine,
  ShieldAlert,
  ShieldCheck,
  Siren,
  Smartphone,
  Ticket,
  Users,
} from "lucide-react";

import { Audiences } from "@/components/landing/audiences";
import { Faq, HeroCards, Logo, Nav } from "@/components/landing/chrome";
import { Continuity } from "@/components/landing/continuity";
import { FareCalculator } from "@/components/landing/fare-calculator";
import { HeroMap } from "@/components/landing/hero-map";
import { LiveTrip } from "@/components/landing/live-trip";
import { CountUp, Reveal, Spotlight } from "@/components/landing/motion";

export const metadata: Metadata = {
  title: { absolute: "Efoy · Shared school and office commutes in Addis Ababa" },
  description:
    "Efoy turns vetted taxi and Yango drivers into scheduled, route-based shuttles for students and civil servants, with live tracking, verified pickups and a guaranteed replacement vehicle.",
};

const PROBLEMS = [
  { icon: Hourglass, title: "Queues at rush hour", text: "Students and civil servants wait for minibus taxis, pay surge-style prices, and still arrive late." },
  { icon: Eye, title: "Parents in the dark", text: "Nobody can see where a child is between the front door and the school gate." },
  { icon: Banknote, title: "School buses cost too much", text: "Private buses are expensive, and most schools and government offices run no fleet of their own." },
  { icon: Car, title: "Drivers sit idle", text: "Taxi and Yango drivers have spare capacity and unpredictable income outside the peaks." },
  { icon: Siren, title: "A breakdown strands everyone", text: "When a hired vehicle fails today, its riders are simply left at the roadside." },
];

const STEPS = [
  { icon: MapPinned, title: "Find your route", text: "Drop a pin at home. Efoy shows the shuttles to your school or office and the nearest stop." },
  { icon: Ticket, title: "Reserve a seat", text: "Pick morning, evening or both, and a weekly, monthly or term plan. The price is quoted up front." },
  { icon: CreditCard, title: "Pay your way", text: "Telebirr, Chapa or a bank transfer. Or your institution pays, in full or in part." },
  { icon: ScanLine, title: "Ride, tracked", text: "Board with a QR code. Guardians get pickup and drop-off alerts and watch the trip live." },
];

const SAFETY = [
  { icon: BadgeCheck, title: "Vetted drivers", text: "Licence, police clearance, photo, vehicle libre, inspection and insurance, all reviewed before a first trip." },
  { icon: QrCode, title: "Verified boarding", text: "Every pickup and drop-off is confirmed by QR, PIN or driver tap inside the stop's geofence." },
  { icon: ShieldCheck, title: "Pre-trip checklist", text: "Fuel, tyres, seatbelts and first aid are checked before the shuttle may start its run." },
  { icon: Users, title: "Authorised pickups", text: "Guardians name who may collect a child, with a photo the driver sees." },
  { icon: ShieldAlert, title: "SOS and incidents", text: "One tap alerts dispatch, who can see the vehicle and call riders and the driver." },
  { icon: Fingerprint, title: "Accountable by design", text: "Two-factor staff sign-in and an audit trail of every sensitive action." },
];

const FAQ: [string, string][] = [
  [
    "What does Efoy (እፎይ) mean?",
    "It is the Amharic sigh of relief: the feeling of knowing your child arrived safely or that your seat to work is waiting. That is the promise of the service.",
  ],
  [
    "Who can ride?",
    "Students at partner schools and universities, and staff at partner government offices and companies. Your institution enrols with Efoy; you subscribe to a route that serves it.",
  ],
  [
    "How is the price calculated?",
    "The driver earns a fixed amount per trip based on distance and time. That cost, plus Efoy's commission, is shared equally by the riders on the shuttle, so the fuller it is, the cheaper every seat. Prices are rounded to 10 ETB and include VAT.",
  ],
  [
    "What happens if the shuttle breaks down?",
    "Dispatch sends the nearest standby vehicle in the zone. The target is 20 minutes, the guarantee 30. If no replacement arrives in time, every affected rider is credited the full price of that trip.",
  ],
  [
    "We don't have smartphones at home. Can we still use it?",
    "Yes. Pickup and drop-off alerts are also sent by SMS, and young children can board with a QR card instead of an app.",
  ],
  [
    "How do drivers join?",
    "Registered taxi and Yango drivers sign up in the driver app, upload their documents and vehicle papers, and are activated once the Efoy team has reviewed them.",
  ],
];

export default function WelcomePage() {
  return (
    <div className="landing bg-white text-slate-900">
      <Nav />

      {/* ------------------------------------------------------------ hero */}
      <section id="top" className="relative isolate flex min-h-[100svh] items-center overflow-hidden bg-[#07111f] text-white">
        <HeroMap />
        <div className="pointer-events-none absolute inset-0 bg-gradient-to-r from-[#07111f] via-[#07111f]/85 to-[#07111f]/10" />
        <div className="pointer-events-none absolute inset-0 bg-[#07111f]/55 sm:hidden" />
        <div className="pointer-events-none absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-[#07111f] to-transparent" />
        <div className="pointer-events-none absolute inset-x-0 top-0 h-32 bg-gradient-to-b from-[#07111f] to-transparent" />
        <HeroCards />

        <div className="relative mx-auto w-full max-w-7xl px-4 pt-28 pb-20 sm:px-6 lg:px-8">
          <div className="max-w-2xl">
            <div className="hero-in glass inline-flex items-center gap-2 rounded-full py-1.5 pr-4 pl-1.5 text-sm text-slate-200" style={{ animationDelay: "100ms" }}>
              <span className="rounded-full bg-teal-400 px-2.5 py-0.5 text-xs font-semibold text-[#07111f]">New</span>
              Now enrolling schools and offices in Addis Ababa
            </div>
            <h1 className="hero-in mt-7 text-5xl leading-[1.02] font-semibold tracking-tight text-balance sm:text-6xl lg:text-7xl" style={{ animationDelay: "220ms" }}>
              The daily commute, <span className="text-gradient">finally dependable.</span>
            </h1>
            <p className="hero-in mt-6 max-w-xl text-lg leading-relaxed text-slate-300 sm:text-xl" style={{ animationDelay: "340ms" }}>
              Efoy turns vetted taxi and Yango drivers into scheduled, route-based shuttles for students and civil servants, with live
              tracking, verified pickups and a guaranteed replacement vehicle if anything goes wrong.
            </p>
            <div className="hero-in mt-9 flex flex-wrap items-center gap-3" style={{ animationDelay: "460ms" }}>
              <Link
                href="/login"
                className="group flex items-center gap-2 rounded-2xl bg-gradient-to-r from-teal-400 to-cyan-400 px-6 py-3.5 font-semibold text-[#07111f] shadow-xl shadow-teal-500/25 transition-all hover:shadow-teal-400/40 hover:brightness-110"
              >
                Sign in to Efoy <ArrowRight className="size-4 transition-transform group-hover:translate-x-1" />
              </Link>
              <a href="#how" className="glass flex items-center gap-2 rounded-2xl px-6 py-3.5 font-semibold text-white transition-colors hover:bg-white/10">
                See how it works
              </a>
            </div>
            <dl className="hero-in mt-14 grid max-w-lg grid-cols-3 gap-6" style={{ animationDelay: "600ms" }}>
              {[
                ["16", "seats share one vehicle's cost"],
                ["3–5 s", "live GPS updates"],
                ["30 min", "replacement guarantee"],
              ].map(([v, l]) => (
                <div key={l} className="border-l border-white/15 pl-4">
                  <dt className="text-2xl font-semibold tracking-tight">{v}</dt>
                  <dd className="mt-1 text-xs leading-snug text-slate-400">{l}</dd>
                </div>
              ))}
            </dl>
          </div>
        </div>
        <a href="#problem" aria-label="Scroll down" className="absolute bottom-8 left-1/2 hidden -translate-x-1/2 sm:block">
          <span className="scroll-hint block h-10 w-6 rounded-full border-2 border-white/30" />
        </a>
      </section>

      {/* ------------------------------------------------------------ marquee */}
      <div className="relative overflow-hidden border-y border-slate-200 bg-slate-50 py-5">
        <div className="marquee flex w-max gap-12 text-sm font-medium whitespace-nowrap text-slate-500">
          {[0, 1].map((k) => (
            <div key={k} className="flex gap-12" aria-hidden={k === 1}>
              {[
                [CreditCard, "Telebirr"],
                [CreditCard, "Chapa"],
                [Banknote, "Bank transfer"],
                [Languages, "English"],
                [Languages, "አማርኛ"],
                [Languages, "Afaan Oromoo"],
                [Smartphone, "Android & iOS"],
                [MessageSquareText, "SMS alerts"],
                [Building2, "Schools · Universities · Ministries"],
                [Radio, "Live GPS"],
              ].map(([Icon, label]) => {
                const I = Icon as typeof CreditCard;
                return (
                  <span key={label as string} className="flex items-center gap-2">
                    <I className="size-4 text-teal-600" /> {label as string}
                  </span>
                );
              })}
            </div>
          ))}
        </div>
      </div>

      {/* ------------------------------------------------------------ problem */}
      <section id="problem" className="relative py-24 sm:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <Reveal className="max-w-3xl">
            <Eyebrow>Why Efoy</Eyebrow>
            <h2 className="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-5xl">
              Twice a day, a whole city <span className="text-teal-600">holds its breath.</span>
            </h2>
            <p className="mt-5 text-lg leading-relaxed text-slate-600">
              Getting to school and to work in Addis Ababa is stressful, costly and unpredictable. Meanwhile thousands of capable vehicles sit
              idle between peaks. Efoy connects the two.
            </p>
          </Reveal>
          <div className="mt-14 grid gap-4 sm:grid-cols-2 lg:grid-cols-6">
            {PROBLEMS.map((p, i) => (
              <Reveal key={p.title} delay={i * 80} className={i < 2 ? "lg:col-span-3" : "lg:col-span-2"}>
                <Spotlight className="group h-full rounded-3xl border border-slate-200 bg-white p-7 transition-shadow hover:shadow-xl hover:shadow-slate-900/5">
                  <span className="grid size-12 place-items-center rounded-2xl bg-rose-50 text-rose-500 transition-transform duration-500 group-hover:scale-110 group-hover:-rotate-6">
                    <p.icon className="size-6" />
                  </span>
                  <h3 className="mt-6 text-lg font-semibold">{p.title}</h3>
                  <p className="mt-2 leading-relaxed text-slate-600">{p.text}</p>
                </Spotlight>
              </Reveal>
            ))}
          </div>
          <Reveal className="mt-6">
            <div className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-teal-500 to-cyan-600 p-8 text-white sm:p-10">
              <div className="grid-bg absolute inset-0 opacity-20" />
              <div className="relative grid items-center gap-6 lg:grid-cols-[1fr_auto]">
                <div>
                  <div className="text-sm font-semibold tracking-wider text-teal-100 uppercase">The Efoy answer</div>
                  <p className="mt-2 max-w-3xl text-2xl leading-snug font-medium text-balance sm:text-3xl">
                    Scheduled shuttles on fixed routes, shared by up to 16 riders, driven by vetted local drivers, and watched over live from
                    the first stop to the gate.
                  </p>
                </div>
                <a href="#how" className="flex items-center gap-2 justify-self-start rounded-2xl bg-white px-5 py-3 font-semibold text-teal-700 shadow-lg">
                  How it works <ArrowRight className="size-4" />
                </a>
              </div>
            </div>
          </Reveal>
        </div>
      </section>

      {/* ------------------------------------------------------------ how */}
      <section id="how" className="relative overflow-hidden bg-slate-50 py-24 sm:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <Reveal className="mx-auto max-w-2xl text-center">
            <Eyebrow>How it works</Eyebrow>
            <h2 className="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-5xl">From a pin on the map to a seat that waits for you.</h2>
          </Reveal>
          <div className="relative mt-16">
            <div className="steps-line absolute top-8 right-[12%] left-[12%] hidden h-0.5 lg:block" />
            <ol className="grid gap-6 lg:grid-cols-4">
              {STEPS.map((s, i) => (
                <Reveal as="li" key={s.title} delay={i * 140} className="relative text-center">
                  <div className="relative mx-auto grid size-16 place-items-center rounded-2xl bg-white text-teal-600 shadow-lg ring-1 shadow-slate-900/5 ring-slate-200">
                    <s.icon className="size-7" />
                    <span className="absolute -top-2 -right-2 grid size-6 place-items-center rounded-full bg-slate-900 text-xs font-semibold text-white">{i + 1}</span>
                  </div>
                  <h3 className="mt-6 text-lg font-semibold">{s.title}</h3>
                  <p className="mx-auto mt-2 max-w-xs leading-relaxed text-slate-600">{s.text}</p>
                </Reveal>
              ))}
            </ol>
          </div>
        </div>
      </section>

      {/* ------------------------------------------------------------ live + guarantee */}
      <section id="live" className="relative overflow-hidden bg-[#07111f] py-24 text-white sm:py-32">
        <div className="aurora" />
        <div className="relative mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <Reveal className="max-w-3xl">
            <Eyebrow dark>Live tracking</Eyebrow>
            <h2 className="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-5xl">Watch the morning run, stop by stop.</h2>
            <p className="mt-5 text-lg leading-relaxed text-slate-400">
              This is a real route across Addis Ababa, planned on the road network. Follow the shuttle as it collects riders, and see exactly
              what a parent sees on their phone.
            </p>
          </Reveal>
          <Reveal className="mt-14" from="scale">
            <LiveTrip />
          </Reveal>

          <div id="guarantee" className="scroll-mt-24 pt-28">
            <Reveal className="max-w-3xl">
              <Eyebrow dark>The continuity guarantee</Eyebrow>
              <h2 className="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-5xl">
                No rider is ever left <span className="text-gradient">at the roadside.</span>
              </h2>
              <p className="mt-5 text-lg leading-relaxed text-slate-400">
                Every zone keeps vetted standby vehicles on call during the peaks. If a shuttle has a problem, the nearest one takes over, and
                guardians are told at every step.
              </p>
            </Reveal>
            <Reveal className="mt-14">
              <Continuity />
            </Reveal>
          </div>

          <div className="mt-24 grid gap-px overflow-hidden rounded-3xl bg-white/10 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { v: 20, suffix: " min", l: "target time to a replacement" },
              { v: 2, suffix: "+", l: "standby vehicles per zone at peak" },
              { v: 100, suffix: "%", l: "of the trip credited if we miss the guarantee" },
              { v: 5, suffix: "", l: "operating zones at launch" },
            ].map((s) => (
              <div key={s.l} className="bg-[#07111f] p-8">
                <div className="text-4xl font-semibold tracking-tight text-white sm:text-5xl">
                  <CountUp to={s.v} />
                  <span className="text-teal-300">{s.suffix}</span>
                </div>
                <div className="mt-2 text-sm text-slate-400">{s.l}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ------------------------------------------------------------ safety */}
      <section className="py-24 sm:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="grid gap-12 lg:grid-cols-[0.9fr_1.1fr] lg:gap-16">
            <Reveal className="lg:sticky lg:top-28 lg:self-start">
              <Eyebrow>Safety first</Eyebrow>
              <h2 className="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-5xl">Built for the most precious passengers.</h2>
              <p className="mt-5 text-lg leading-relaxed text-slate-600">
                Safety is not a feature on top; it is how every part of Efoy works, from who may drive to how a child gets on and off.
              </p>
              <div className="mt-8 flex items-center gap-4 rounded-2xl border border-slate-200 bg-slate-50 p-5">
                <span className="grid size-12 shrink-0 place-items-center rounded-xl bg-teal-500 text-white">
                  <Bell className="size-6" />
                </span>
                <p className="text-sm leading-relaxed text-slate-600">
                  <span className="font-semibold text-slate-900">Not scanned within 3 minutes?</span> The driver marks a no-show and the guardian
                  is told straight away.
                </p>
              </div>
            </Reveal>
            <div className="grid gap-4 sm:grid-cols-2">
              {SAFETY.map((s, i) => (
                <Reveal key={s.title} delay={(i % 2) * 100}>
                  <Spotlight className="group h-full rounded-3xl border border-slate-200 bg-white p-6 transition-all hover:-translate-y-1 hover:shadow-xl hover:shadow-slate-900/5">
                    <span className="grid size-11 place-items-center rounded-xl bg-teal-50 text-teal-600 transition-colors group-hover:bg-teal-500 group-hover:text-white">
                      <s.icon className="size-5" />
                    </span>
                    <h3 className="mt-5 font-semibold">{s.title}</h3>
                    <p className="mt-2 text-sm leading-relaxed text-slate-600">{s.text}</p>
                  </Spotlight>
                </Reveal>
              ))}
            </div>
          </div>
        </div>
      </section>

      {/* ------------------------------------------------------------ pricing */}
      <section id="pricing" className="relative overflow-hidden bg-slate-50 py-24 sm:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <Reveal className="mx-auto max-w-3xl text-center">
            <Eyebrow>Fair, shared pricing</Eyebrow>
            <h2 className="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-5xl">The fuller the shuttle, the less everyone pays.</h2>
            <p className="mt-5 text-lg leading-relaxed text-slate-600">
              The driver earns a fixed amount per trip. That cost is split equally across the seats. Move the sliders to see how it works.
            </p>
          </Reveal>
          <Reveal className="mt-14" from="scale">
            <FareCalculator />
          </Reveal>
          <p className="mt-6 text-center text-xs text-slate-500">
            Illustrative minibus rates for a monthly plan of 22 working days. Real prices depend on the route and are quoted before you pay. VAT
            included.
          </p>
        </div>
      </section>

      {/* ------------------------------------------------------------ audiences */}
      <section id="audiences" className="py-24 sm:py-32">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
          <Reveal className="mx-auto max-w-2xl text-center">
            <Eyebrow>Made for everyone on the route</Eyebrow>
            <h2 className="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-5xl">One platform, five points of view.</h2>
          </Reveal>
          <Reveal className="mt-12">
            <Audiences />
          </Reveal>
        </div>
      </section>

      {/* ------------------------------------------------------------ faq */}
      <section id="faq" className="bg-slate-50 py-24 sm:py-32">
        <div className="mx-auto grid max-w-7xl gap-12 px-4 sm:px-6 lg:grid-cols-[0.8fr_1.2fr] lg:px-8">
          <Reveal>
            <Eyebrow>Questions</Eyebrow>
            <h2 className="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-5xl">Good to know.</h2>
            <p className="mt-5 text-lg leading-relaxed text-slate-600">Can&apos;t find your answer? Your school or office coordinator can help, or write to us.</p>
            <a href="mailto:hello@efoy.et" className="mt-6 inline-flex items-center gap-2 font-semibold text-teal-700 hover:text-teal-800">
              <LifeBuoy className="size-4" /> hello@efoy.et
            </a>
          </Reveal>
          <Reveal delay={120}>
            <Faq items={FAQ} />
          </Reveal>
        </div>
      </section>

      {/* ------------------------------------------------------------ CTA */}
      <section className="relative overflow-hidden bg-[#07111f] py-24 text-white sm:py-28">
        <div className="aurora" />
        <div className="grid-bg absolute inset-0 opacity-30" />
        <Reveal className="relative mx-auto max-w-4xl px-4 text-center sm:px-6">
          <div className="mx-auto mb-6 grid size-16 place-items-center rounded-2xl bg-white/5 ring-1 ring-white/10">
            <CalendarClock className="size-8 text-teal-300" />
          </div>
          <h2 className="text-4xl font-semibold tracking-tight text-balance sm:text-6xl">
            Bring <span className="text-gradient">relief</span> to your school or office.
          </h2>
          <p className="mx-auto mt-6 max-w-2xl text-lg text-slate-400">
            Institutions, dispatchers and fleet partners manage everything from the Efoy console. Riders and guardians use the Efoy app.
          </p>
          <div className="mt-10 flex flex-wrap justify-center gap-3">
            <Link
              href="/login"
              className="group flex items-center gap-2 rounded-2xl bg-gradient-to-r from-teal-400 to-cyan-400 px-7 py-4 font-semibold text-[#07111f] shadow-xl shadow-teal-500/25 hover:brightness-110"
            >
              Sign in to the console <ArrowRight className="size-4 transition-transform group-hover:translate-x-1" />
            </Link>
            <a href="mailto:hello@efoy.et" className="glass rounded-2xl px-7 py-4 font-semibold hover:bg-white/10">
              Partner with us
            </a>
          </div>
        </Reveal>
      </section>

      <footer className="bg-[#050c17] py-14 text-slate-400">
        <div className="mx-auto grid max-w-7xl gap-10 px-4 sm:px-6 md:grid-cols-[1.4fr_1fr_1fr_1fr] lg:px-8">
          <div>
            <Logo light />
            <p className="mt-4 max-w-xs text-sm leading-relaxed">Shared, scheduled and tracked commutes for students and civil servants. Made in Addis Ababa.</p>
          </div>
          {[
            ["Product", [["How it works", "#how"], ["Live tracking", "#live"], ["Guarantee", "#guarantee"], ["Pricing", "#pricing"]]],
            ["For", [["Parents", "#audiences"], ["Institutions", "#audiences"], ["Drivers", "#audiences"], ["Operations", "#audiences"]]],
            ["Company", [["Sign in", "/login"], ["Contact", "mailto:hello@efoy.et"], ["FAQ", "#faq"]]],
          ].map(([title, links]) => (
            <div key={title as string}>
              <div className="text-sm font-semibold text-white">{title as string}</div>
              <ul className="mt-4 space-y-2.5 text-sm">
                {(links as string[][]).map(([l, h]) => (
                  <li key={l}>
                    <a href={h} className="transition-colors hover:text-teal-300">
                      {l}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <div className="mx-auto mt-12 flex max-w-7xl flex-wrap items-center justify-between gap-4 border-t border-white/5 px-4 pt-8 text-xs sm:px-6 lg:px-8">
          <span>© 2026 Efoy. All rights reserved.</span>
          <span>Map data © OpenStreetMap contributors</span>
        </div>
      </footer>
    </div>
  );
}

function Eyebrow({ children, dark }: { children: React.ReactNode; dark?: boolean }) {
  return (
    <span
      className={
        dark
          ? "inline-flex items-center gap-2 rounded-full bg-teal-400/10 px-3 py-1 text-xs font-semibold tracking-wider text-teal-300 uppercase ring-1 ring-teal-400/20"
          : "inline-flex items-center gap-2 rounded-full bg-teal-50 px-3 py-1 text-xs font-semibold tracking-wider text-teal-700 uppercase ring-1 ring-teal-600/10"
      }
    >
      <span className={dark ? "size-1.5 rounded-full bg-teal-300" : "size-1.5 rounded-full bg-teal-500"} />
      {children}
    </span>
  );
}
