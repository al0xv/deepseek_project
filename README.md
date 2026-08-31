# DeepSeek Terminal

Временный интерактивный чат с DeepSeek из терминала любого Windows/macOS
компьютера, подтверждаемый с вашего iPhone (сканирование QR + Face ID).
**DeepSeek API key никогда не покидает доверенное устройство** и не попадает
в RAM чужого ПК, его shell history, логи или файлы. История сессии живёт
только в RAM gateway и уничтожается вместе с сессией — никакой БД, никакого
persistence.

- **`ds`** — терминальный клиент (не знает API key, ничего не сохраняет).
- **`dsgateway`** — gateway: единственный компонент, который держит API key.
- **DS Remote (iOS)** — контроллер: сканирует QR, подтверждает Face ID,
  завершает сессию (End Session).

## Стадия проекта

Реализовано и проверено: **MVP0–MVP2, Phase 2.4, Phase 3.1, Phase 3.2,
Phase 3.2.1, Phase 3.3A** (см. `docs/roadmap.md`).

| Проверка | Статус |
|----------|--------|
| Go build / vet / test | **PASS** (автоматически, каждый этап) |
| Windows amd64 cross-build | **PASS** (автоматически) |
| Linux amd64/arm64 static build | **PASS** (автоматически) |
| iOS unit tests (simulator) | **PASS** (автоматически) |
| Real iPhone QR / Face ID / approve / End Session | **PASS** (физический iPhone) |
| Real DeepSeek API / streaming / multi-turn | **PASS** (реальный платный запрос) |
| Real iPhone + DeepSeek control flow | **PASS** |
| Real Windows machine test | `AWAITING` (бинарь собран, ручная проверка не проведена) |
| Public Oracle VM mock E2E (Phase 3.3B) | `AWAITING` (нужна реальная VM) |

### Явные заглушки (stub) — не работают намеренно

| Функция | Фаза | Поведение сегодня |
|---------|------|-------------------|
| Доставка session-scoped DeepSeek API key с iPhone в gateway | Phase 3.4 | iOS хранит ключ только в Keychain и **не отправляет** его; gateway возвращает **`501 not_implemented`** на любой `api_key` в `/v1/pair`. Рабочий способ: `DEEPSEEK_API_KEY` на gateway. |
| Публичный gateway с реальным DeepSeek | Phase 3.4 | Публичный деплой — только mock-провайдер (без API key, без трат). |
| SSH-вход / one-command UX | Phase 3.5 | Не реализовано. |
| Постоянный домен вместо `sslip.io` | Phase 3.6 | Не реализовано. |

Ни одна заглушка не маскируется под рабочую функцию: протокол либо отклоняет
запрос с явной ошибкой, либо функция задокументирована как `AWAITING`.

## Product defaults (per-session)

- **Model:** DeepSeek V4 Flash (`deepseek-v4-flash`)
- **Thinking:** enabled
- **Reasoning effort:** high

Доступны: V4 Flash / V4 Pro; Thinking Off / Low / High / Max. Настройки
выбираются на iPhone перед approve, живут в RAM сессии, immutable после
approval и показываются терминалом как
`DeepSeek V4 Flash · Thinking High`. `DS_MODEL` остаётся dev-override для
default-модели gateway (если iPhone не указал модель).

## Known fixed bug

Первый физический тест на iPhone обнаружил crash при обращении к Face ID без
`NSFaceIDUsageDescription` («crashed because it attempted to access privacy
sensitive data without a usage description»). Фикс персистентно закреплён в
`ios/Info.plist` (source of truth; переживает xcodegen-регенерацию).

## Возможности

- `ds` — терминальный клиент (не знает API key).
- `dsgateway` — gateway (единственный компонент с API key), in-memory сессии.
- Pairing: `WAITING → APPROVED → ACTIVE → DESTROYED`, QR + 6-значный код,
  single-use, истечение 120s.
- Стриминг ответа (SSE), многоходовый диалог, `/exit`, Ctrl+C (отмена
  генерации / выход).
- Нет БД, нет файлов истории, нет persistence.

## Сборка

```bash
make build          # macOS: bin/ds, bin/dsgateway
make build-windows  # Windows: bin/ds.exe, bin/dsgateway.exe
```

## Запуск (два варианта)

### С реальным DeepSeek API key

```bash
export DEEPSEEK_API_KEY=sk-...
./bin/dsgateway                       # на доверенной машине

./bin/ds --remote http://localhost:8080   # клиент (в другом терминале)
# ... WAITING + QR + Code: 472 913 ...
./bin/dsgateway approve 472913        # одобрить сессию (loopback)
# клиент: ✓ Approved → чат → /exit
```

### Без API key (mock, для демо/разработки)

```bash
./bin/dsgateway -mock
./bin/ds --remote http://localhost:8080
./bin/dsgateway approve <code>
```

## Real Windows LAN test

Валидация Windows→Mac через локальную сеть. Windows запускает только
готовый `ds.exe` — без Go, без API key, без установки.

### На Mac (gateway)

```bash
# 1. собрать артефакты
make build build-windows

# 2. запустить gateway, доступный из LAN (только для теста)
./bin/dsgateway -mock -listen 0.0.0.0:8080

# 3. узнать LAN IP Mac (например)
ipconfig getifaddr en0
# → 192.168.1.42
```

> Default `dsgateway` без флагов слушает только `127.0.0.1:8080`.
> Для LAN-теста bind на `0.0.0.0` обязателен и явный.

### На Windows (client)

```powershell
# скопировать bin\ds.exe, затем:
.\ds.exe --remote http://192.168.1.42:8080
```

Ожидаемый вывод:

```
QR
Code: XXX XXX
Waiting for approval...
```

### Approve (на Mac, loopback)

```bash
./bin/dsgateway approve XXXXXX
```

### На Windows

```
✓ Approved

You > hello world
```

Ожидаемый ответ (mock стримится по кускам):

```
DeepSeek > mock reply to: hello world
```

Далее:

```
You > /exit
```

Ожидаемый вывод:

```
Session destroyed.
```

После этого reconnect/resume этой сессии невозможен.

### Проверяемые failure cases (Windows)

1. **Gateway недоступен**: `.\ds.exe --remote http://192.168.1.99:8080` —
   клиент завершается с понятной ошибкой (без panic/stack trace).
2. **Pairing не одобрен**: prompt до approve возвращает
   `pairing_pending` / `not_approved` — сессия остаётся заблокирована.
3. **Gateway убит во время сессии**: клиент печатает `Error: ...` и
   возвращается в безопасное состояние (prompt). Reconnect/resume
   намеренно не поддерживается.

## iPhone manual pairing development flow

Замена `./bin/dsgateway approve XXXXXX` на подтверждение с iPhone
(приложение DS Remote, `ios/`).

1. **Mac**: `./bin/dsgateway -mock -listen 0.0.0.0:8080`
2. **Terminal**: `./bin/ds --remote http://<MAC_IP>:8080` → `Code: XXX XXX`
3. **iPhone**: DS Remote → указать `http://<MAC_IP>:8080` → **сканировать QR**
   в терминале (или ввести код вручную) → **Approve** → Face ID
4. **Terminal**: `✓ Approved` → чат (mock)
5. **iPhone**: **End Session** → сессия уничтожается (`DELETE /v1/sessions/{id}`
   c `Authorization: Bearer <control_token>`)
6. **Terminal** при следующем prompt: `Error: not_found: session not found`

Детали: `ios/README.md`. Ручной ввод 6-значного кода остаётся fallback'ом.

## Real DeepSeek manual verification

Проверка реального DeepSeek API на Mac + физическом iPhone (без mock).

### Terminal 1 — gateway (trusted Mac)

```bash
export DEEPSEEK_API_KEY='MY_REAL_KEY'   # не печатать ключ в history, если возможно
./bin/dsgateway -listen 0.0.0.0:8080
```

Если ключа нет — gateway завершится с понятной ошибкой
(`DEEPSEEK_API_KEY is not set`). LAN IP:

```bash
ipconfig getifaddr en0        # например 192.168.20.13
```

### Terminal 2 — DS client

```bash
./bin/ds --remote http://127.0.0.1:8080
```

Ожидание: `QR`, `Code: XXX XXX`, `Waiting for approval...`.

### Физический iPhone

DS Remote → Gateway = `http://<MAC_LAN_IP>:8080` → **Scan QR** → Face ID →
`ACTIVE SESSION`. Терминал: `✓ Approved`, `DeepSeek ready.`, `You >`.

Затем реальный prompt, например:

```
Привет. Ответь одной короткой фразой и скажи, что ты работаешь через DeepSeek API.
```

Ожидание: ответ **не** `mock reply to: ...`, реальная модель, стриминг.
Второй ход: `А какой был мой предыдущий вопрос?` — модель показывает
multi-turn контекст. Затем на iPhone **End Session** → следующий prompt в
терминале: `Error: not_found: session not found`.

### Настройки модели (Test A–D)

Выбираются на iPhone на экране Pairing (Model / Thinking) перед Scan QR.
Терминал показывает canonical значения после `✓ Approved`.

- **Test A — Flash / High** (default): терминал
  `DeepSeek V4 Flash · Thinking High`. Prompt: `Ответь одним словом: FLASH`
  → реальный ответ DeepSeek.
- **Test B — Flash / Off**: новая сессия, терминал
  `DeepSeek V4 Flash · Thinking Off`; реальный API работает.
- **Test C — Pro / High**: новая сессия, терминал
  `DeepSeek V4 Pro · Thinking High`; один короткий prompt (экономия cost).
- **Test D — Multi-turn** на Flash/High: два хода — контекст сохраняется.

> Правило cost: автоматические тесты не ходят в платный API (используют
> mock HTTP server); ручная проверка — только короткими prompt'ами.

## Oracle Always Free deployment

Публичное развёртывание `dsgateway` на одной Oracle Cloud Always Free VM
(тестовый, **mock-only**, без API key) — см. **`deploy/oci/README.md`**.

Кратко:

```bash
make build-linux            # bin/dsgateway-linux-amd64 / -arm64
./deploy/oci/deploy.sh \
  --ip 203.0.113.42 \
  --ssh-host 203.0.113.42 \
  --ssh-key ~/.ssh/oracle_vm_key \
  --arch amd64
```

Схема: `ds / iPhone → HTTPS :443 → Caddy → 127.0.0.1:8080 → dsgateway -mock`.
Порт `8080` публично **не** открывается. Временный hostname — `sslip.io`.
`Local development` (выше) остаётся без изменений.

## Переменные окружения

| Переменная | Значение по умолчанию | Описание |
|-----------|----------------------|----------|
| `DEEPSEEK_API_KEY` | — | DeepSeek API key (только gateway) |
| `DS_GATEWAY_URL` | `http://localhost:8080` | базовый URL gateway для клиента и approve |
| `DS_GATEWAY_ADDR` | `127.0.0.1:8080` | адрес прослушивания gateway (loopback) |
| `-listen` (флаг) | `127.0.0.1:8080` | адрес прослушивания, напр. `0.0.0.0:8080` для LAN-тестов |
| `DS_MODEL` | `deepseek-v4-flash` | модель gateway по умолчанию |
| `DS_PAIR_TIMEOUT` | `120s` | время жизни pairing |
| `DS_IDLE_TIMEOUT` | `5m` | idle-таймаут сессии |
| `DS_GEN_TIMEOUT` | `60s` | таймаут генерации |
| `DS_MAX_SESSIONS` | `8` | максимум одновременных сессий |

## Протокол и архитектура

См. `docs/architecture.md`, `docs/protocol.md`, `docs/security.md`.

## Тесты

```bash
go test ./...
```
