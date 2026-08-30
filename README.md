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

## Переменные окружения

| Переменная | Значение по умолчанию | Описание |
|-----------|----------------------|----------|
| `DEEPSEEK_API_KEY` | — | DeepSeek API key (только gateway) |
| `DS_GATEWAY_URL` | `http://localhost:8080` | базовый URL gateway для клиента и approve |
| `DS_GATEWAY_ADDR` | `:8080` | адрес прослушивания gateway |
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
