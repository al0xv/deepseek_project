# Client/Gateway Protocol

Все сообщения — JSON. Стрим ответа — SSE (`text/event-stream`,
`Cache-Control: no-cache`, `X-Accel-Buffering: no`).

## Эндпоинты

| Метод | Путь | Тело | Ответ |
|-------|------|------|-------|
| `POST` | `/v1/sessions` | `SessionCreateRequest` | `SessionCreateResponse` (201) |
| `GET` | `/v1/sessions/{id}` | — | `SessionStatusResponse` |
| `POST` | `/v1/sessions/{id}/prompt` | `PromptRequest` | SSE: `token`, `done`, `error` |
| `POST` | `/v1/sessions/{id}/cancel` | — | 204 |
| `POST` | `/v1/sessions/{id}/close` | — | 204 |
| `POST` | `/v1/pair` | `ApproveRequest` | `ApproveResponse` |

## Сообщения

```jsonc
// POST /v1/sessions → 201
{
  "session_id": "c8f3...",
  "pairing_token": "yusMGYKJFR-d4XsSgRHZHfDmd8EjWkE6mtcI1svmedQ",
  "pairing_code": "472 913",
  "gateway_url": "http://localhost:8080",
  "expires_in": 120
}

// GET /v1/sessions/{id} → 200
{ "session_id": "c8f3...", "state": "WAITING" }  // WAITING|APPROVED|ACTIVE

// POST /v1/sessions/{id}/prompt
{ "content": "объясни Python decorators" }

// POST /v1/pair — одно из двух:
{ "pairing_token": "yusMGYK..." }   // из QR
{ "pairing_code": "472913" }        // ручной ввод
```

## SSE-события стрима

```
event: token
data: {"delta":"Python"}

event: token
data: {"delta":" decorators"}

event: done
data: {"finish_reason":"stop"}       // stop | length | cancelled

event: error                          // только при ошибке
data: {"code":"upstream_error","message":"..."}
```

## QR-содержимое

QR кодирует JSON-строку (не содержит API key):

```json
{"v":1,"session_id":"c8f3...","pairing_token":"eyJ...","gateway_url":"https://192.168.1.5:8443"}
```

## Lifecycle одного сообщения

1. `POST /v1/sessions` → 201 (session_id, token, code, gateway_url).
2. Клиент печатает WAITING + QR + `Code: 472 913`, опрашивает статус (500ms).
3. `POST /v1/pair` с token/code → сессия APPROVED (single-use, 120s expiry).
4. `POST /v1/sessions/{id}/prompt` → gateway: state=ACTIVE, вызывает провайдера
   с полной историей, стримит `token` события, в конце `done`.
5. История обновляется (user + assistant), state=APPROVED.
6. `/exit` → `POST /v1/sessions/{id}/close` → 204 → session destroyed.
   Также destroy: idle timeout, pair expiry, disconnect, cancel с телефона.

## Ошибки

Не-200 ответы и SSE-событие `error` используют:

```json
{ "code": "pairing_pending" | "pairing_expired" | "already_approved"
| "not_approved" | "not_found" | "too_many_sessions"
| "generation_timeout" | "upstream_error" | "bad_request",
  "message": "..." }
```

## State machine

```
WAITING ──approve──► APPROVED ──prompt──► ACTIVE
   │                    ▲  │                 │
   │ pair expiry        └──┘ done/cancel     │
   ▼                                          ▼
DESTROYED ◄────── /exit, close, disconnect, idle, timeout, error
```

После DESTROYED сессия невосстановима. Reconnect/resume не поддерживается.
