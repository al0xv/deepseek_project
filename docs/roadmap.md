# Project Roadmap

Статусы верификации на текущий момент: см. `README.md` (REAL IPHONE/REAL
DEEPSEEK — PASS; REAL WINDOWS — AWAITING). Ниже — только дорожная карта;
фазы, помеченные как будущие, **не реализованы**.

## Phase 3.2.1 — Model & Thinking Selection

CURRENT PHASE (реализована).

```
Model: V4 Flash / V4 Pro
Thinking: Off / Low / High / Max
```

Default: **V4 Flash / High**. Settings per-session, immutable после approval,
RAM-only; терминал показывает canonical значения после approve.

---

## Phase 3.2.2 — Real Device UX Hardening

Goal: verify the now-complete everyday local workflow on real devices.

Required:

- physical iPhone regression test;
- physical Windows test;
- physical Mac test;
- QR readability in Windows Terminal;
- manual-code fallback;
- gateway unavailable UX;
- network disconnect UX;
- expired pairing UX;
- Face ID cancel UX;
- End Session UX.

This phase prioritizes bug fixes only. No new major architecture.
Important milestone: **REAL WINDOWS VERIFICATION: PASS**.

---

## Phase 3.3 — Secure API Credential Transport DESIGN

IMPORTANT: не отправлять DeepSeek API key с iPhone на текущий gateway —
текущее LAN-соединение `http://<Mac-IP>:8080` передаёт key в открытом виде,
что неприемлемо. Фаза начинается с security design checkpoint'а.

Goal: `iPhone Keychain → authenticated encrypted channel → gateway RAM →
DeepSeek`.

Requirements:

- API key никогда не достигает terminal client;
- никогда не в QR;
- никогда в логах;
- никогда не персистится на gateway;
- network attacker не может прочитать key;
- terminal PC не может обманом заставить телефон отправить key на произвольный endpoint.

Compare at least:

A. normal HTTPS gateway with trusted certificate/domain;
B. pinned gateway public key / certificate;
C. authenticated application-layer encrypted transport;
D. delay key-delivery until a public trusted HTTPS gateway exists.

Do NOT implement before an explicit security decision. Prefer the likely
simplest production choice over custom cryptography.

---

## Phase 3.4 — Session-scoped API Key Delivery

Only after Phase 3.3 security design is approved.

Goal: DeepSeek key permanently stored only in iPhone Keychain; during an
approved session `iPhone → secure transport → gateway RAM → DeepSeek`; on
destroy: clear key reference / messages / pairing credentials / control token.
Mac env should no longer require `DEEPSEEK_API_KEY` for the normal flow.
Development env fallback may remain.

---

## Phase 3.5 — Public Gateway Without Buying Infrastructure First

Goal: `school Windows/Mac → Internet → gateway` without requiring same LAN.
Research FIRST; prioritize free tier / zero-cost; HTTPS; streaming;
short-lived in-memory sessions; reasonable rate limiting; no persistent
conversation DB; support the iPhone controller. Compare realistic current
hosting options with up-to-date docs. If no reliable free option, quantify the
cheapest VPS alternative before purchasing.

---

## Phase 3.6 — Zero/Minimal Install Foreign Computer UX

Original core product goal:

```
approach Windows/Mac → open terminal → one short command → QR → iPhone → DeepSeek
```

Investigate after public gateway exists. Preferred final UX candidate:
`ssh dsh.example` if public SSH is clean; alternative Windows bootstrap:
short PowerShell command; macOS: short curl command. Goals: no API key, no
permanent config, no history, no installer if avoidable, temporary client only.
Windows #1, macOS #2, Linux out of scope.

---

## Phase 3.7 — Session UX Polish

Only after connectivity architecture is stable. Potential optional features:
session timer; selected model shown on phone/terminal; End Session improvement;
immediate terminal notification when iPhone kills session; optional
cost/tokens for current session; optional max-session-duration control;
clearer offline/errors.

---

## Phase 4 — First Usable Release

Definition: on arbitrary real Windows or Mac: `one short command → QR →
physical iPhone → Face ID → DeepSeek V4 Flash/Pro → streamed multi-turn →
End Session → complete cleanup`. Security: key on iPhone; foreign computer
never receives key; gateway uses encrypted trusted connection; sessions
ephemeral; no chat persistence. Validation: REAL IPHONE / REAL WINDOWS /
REAL MAC / REAL DEEPSEEK — PASS.

---

## Explicitly Deferred Beyond First Release

Codex clone; shell execution; arbitrary local commands; filesystem read/write;
git; VS Code extension; remote Mac control; coding-agent tools; voice control;
chat sync/history; Android; Linux. Evaluate only after the basic
temporary-terminal product is genuinely useful.
