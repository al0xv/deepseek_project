# Project Roadmap

Статусы верификации на текущий момент: см. `README.md` (REAL IPHONE /
REAL DEEPSEEK — PASS; REAL WINDOWS — AWAITING; REAL ORACLE — AWAITING).
Ниже — только дорожная карта; фазы, помеченные как будущие, **не реализованы**.

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

## Phase 3.3A — Oracle Always Free Deployment Preparation

CURRENT PHASE (реализована — подготовка репозитория).

- Linux build targets (amd64/arm64, static, `CGO_ENABLED=0`).
- `deploy/oci/`: install.sh / deploy.sh / run-gateway.sh / dsgateway.service /
  Caddyfile.template / README.
- Health endpoint `GET /healthz`.
- Первый публичный deployment — только `-mock` (без API key, без трат).
- `dsgateway` слушает только `127.0.0.1:8080`; публичный вход — только Caddy :443.
- Временный free hostname `sslip.io`; обычный домен позже заменяет его без
  изменения кода gateway.
- Cм. `deploy/oci/README.md`.

## Phase 3.3B — Public Oracle Mock E2E Verification

(manual; требует реальной OCI VM пользователя)

Проверка публичного mock-флоу: `ds --remote https://<hostname>` → QR →
iPhone Scan QR → Face ID → approve → `DeepSeek V4 Flash · Thinking High` →
`mock reply to: hello` → End Session → `not_found`. Плюс negative tests
(порт 8080 недоступен публично, wrong/expired pairing, wrong control token).

Статус: `AWAITING REAL ORACLE VM VERIFICATION`.

---

## Phase 3.4 — Secure iPhone API-key Delivery over Public HTTPS

Только после успешной 3.3B. Goal:

```
iPhone Keychain → HTTPS → Oracle gateway RAM → DeepSeek
```

Требования: key не достигает terminal client; не в QR; не в логах; не
персистится на gateway; network attacker не может прочитать; terminal PC не
может заставить телефон отправить key на произвольный endpoint. Начать с
security decision (trusted cert vs pinning vs app-layer encryption); отдавать
предпочтение простому production-варианту, а не самодельной криптографии.
Mac env больше не требует `DEEPSEEK_API_KEY` в normal flow (dev fallback
может остаться).

---

## Phase 3.5 — SSH terminal entrypoint / one-command UX

Original product goal: `approach Windows/Mac → one short command → QR →
iPhone → DeepSeek`. Рассмотреть `ssh dsh.example`, альтернативы Windows
(short PowerShell) и macOS (short curl). Без API key, без постоянного
конфига, без истории, без installer если возможно, temporary client only.
Windows #1, macOS #2, Linux out of scope.

---

## Phase 3.6 — Final hostname / no-install UX polish

Замена `sslip.io` на постоянный hostname; полировка one-command bootstrap и
no-install UX.

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
chat sync/history; Android; Linux end-user support (Linux — server-only).
Evaluate only after the basic temporary-terminal product is genuinely useful.

