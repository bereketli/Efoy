"use client";

import { useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { Problem } from "@/lib/auth/upstream";

type Step = "password" | "totp";

// Two steps: email + password first; if core-api answers TOTP_REQUIRED, ask
// for the authenticator code and submit all three.
export function LoginForm({ next }: { next: string }) {
  const router = useRouter();
  const [step, setStep] = useState<Step>("password");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const codeInput = useRef<HTMLInputElement>(null);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError(null);

    let res: Response;
    try {
      res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ email, password, ...(step === "totp" ? { totp_code: code } : {}) }),
      });
    } catch {
      setError("Cannot reach the server. Check your connection and try again.");
      setPending(false);
      return;
    }

    if (res.ok) {
      router.replace(next);
      router.refresh();
      return;
    }

    const problem = (await res.json().catch(() => null)) as Problem | null;
    setPending(false);
    if (problem?.code === "TOTP_REQUIRED") {
      setStep("totp");
      setTimeout(() => codeInput.current?.focus(), 0);
      return;
    }
    if (problem?.code === "TOTP_INVALID") setCode("");
    setError(problem?.detail ?? problem?.title ?? "Sign-in failed. Try again.");
  }

  function startOver() {
    setStep("password");
    setPassword("");
    setCode("");
    setError(null);
  }

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-4" noValidate>
      <div className="flex flex-col gap-2">
        <Label htmlFor="email">Email</Label>
        <Input
          id="email"
          type="email"
          autoComplete="username"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          readOnly={step === "totp"}
          autoFocus
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label htmlFor="password">Password</Label>
        <Input
          id="password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          readOnly={step === "totp"}
        />
      </div>

      {step === "totp" && (
        <div className="flex flex-col gap-2">
          <Label htmlFor="totp">Authenticator code</Label>
          <Input
            id="totp"
            ref={codeInput}
            inputMode="numeric"
            autoComplete="one-time-code"
            pattern="[0-9]{6}"
            maxLength={6}
            placeholder="123456"
            required
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
          />
          <p className="text-muted-foreground text-xs">Enter the 6-digit code from your authenticator app.</p>
        </div>
      )}

      {error && (
        <p role="alert" className="text-destructive text-sm">
          {error}
        </p>
      )}

      <Button type="submit" disabled={pending || (step === "totp" && code.length !== 6)}>
        {pending ? "Signing in…" : step === "totp" ? "Verify and sign in" : "Continue"}
      </Button>
      {step === "totp" && (
        <Button type="button" variant="link" size="sm" onClick={startOver}>
          Use a different account
        </Button>
      )}
    </form>
  );
}
