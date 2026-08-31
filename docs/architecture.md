# Architecture

Реализованный статус: **MVP0–MVP2, Phase 2.4, Phase 3.1–3.2.1, Phase 3.3A**
(подготовка публичного деплоя). Фазы 3.3B–3.6 — будущие; см. `roadmap.md`.

## Executive decisions

| Решение | Выбор | Why |
|---------|-------|-----|
| Язык | **Go** (клиент и gateway) | один статический бинарник, кросскомпиляция Windows/macOS, stdlib покрывает HTTP/SSE/крипто/таймауты |
| Transport client↔gateway | **HTTP + SSE** (`net/http` + `http.Flusher`) | однонаправленный стрим, ноль зависимостей, проще WebSocket/raw TCP |
| SSH | **не используется в MVP0–2** | лишняя сложность; позже возможен только как опциональный туннель |
| MVP0 | один локальный CLI → DeepSeek | быстрая проверка UX |
| Persistence | **нет** | только RAM, destroy полностью освобождает |
| Pairing | crypto/rand token (32B) + 6-значный код, single-use, 120s | защита от перехвата/угадывания |
| TLS | **появляется в MVP3** | требуется для iOS ATS; в MVP1–2 только localhost/dev |

## Структура

```
cmd/ds             терминальный клиент (не знает API key)
cmd/dsgateway      gateway (только он держит key) + `approve` subcommand
internal/config    чтение env
internal/provider  Provider interface (минимальная абстракция над OpenAI-compatible API)
internal/provider/deepseek  реализация DeepSeek (SSE upstream)
internal/provider/mock      фейковый провайдер для разработки/тестов
internal/crypto    session id, pairing token, 6-значный code
internal/protocol  wire-типы + SSE encode/decode
internal/session   Session + SessionManager (in-memory state machine, Clock-инъекция)
internal/gateway   HTTP handlers, SSE стриминг, rate limit
internal/client    GatewayClient (HTTP/SSE) + REPL (терминальный UI)
tests/integration  e2e через реальный gateway + transport
docs/              этот документ, protocol.md, security.md
```

## Поток

```
ds (клиент) ──HTTP/SSE──► dsgateway ──HTTPS──► DeepSeek API
                                │
                                └── iPhone (MVP3): scan QR → approve/end
```

Клиент отправляет только `{content}`; gateway хранит conversation history в RAM
и шлёт полную историю провайдеру. Провайдер инкапсулирован за
`provider.Provider`, поэтому позже подключаются OpenAI и другие без переделки
core.

## Фазы

- **MVP0**: `ds` напрямую с DeepSeek (интерактивный стриминг, `/exit`, Ctrl+C).
- **MVP1**: split на `ds` + `dsgateway`; ключ уходит из клиента; SSE; таймауты;
  disconnect → destroy; e2e.
- **MVP2**: pairing (WAITING→APPROVED), QR + code, `dsgateway approve`,
  полный state machine.
- **MVP3** (реализован, Phase 3.1–3.2): iPhone (Keychain, scan QR, Face ID,
  approve/end), `control_token` для End Session, per-session model & thinking.
  TLS на gateway и доставка key с iPhone в RAM gateway — **Phase 3.4** (stub).

## Ключевые ограничения (по требованиям)

- Никакой БД, никакой persistent истории, никаких файлов на чужом ПК.
- API key не в командах, не в env клиента, не в логах, не в QR.
- Сессия после DESTROYED невосстановима (нет resume/reconnect).
