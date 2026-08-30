# DeepSeek Terminal

Личный инструмент для временного интерактивного чата с DeepSeek из терминала
чужого Windows/macOS компьютера. API key никогда не покидает доверенное
устройство. История живёт только в RAM и уничтожается вместе с сессией.

Статус: **MVP2 реализован** (клиент + gateway + pairing с QR). MVP3 (iOS)
— задокументирован как будущий этап.

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

## Переменные окружения

| Переменная | Значение по умолчанию | Описание |
|-----------|----------------------|----------|
| `DEEPSEEK_API_KEY` | — | DeepSeek API key (только gateway) |
| `DS_GATEWAY_URL` | `http://localhost:8080` | базовый URL gateway для клиента и approve |
| `DS_GATEWAY_ADDR` | `127.0.0.1:8080` | адрес прослушивания gateway (loopback) |
| `-listen` (флаг) | `127.0.0.1:8080` | адрес прослушивания, напр. `0.0.0.0:8080` для LAN-тестов |
| `DS_MODEL` | `deepseek-chat` | модель |
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
