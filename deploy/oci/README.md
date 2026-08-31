# Oracle Cloud Always Free — dsgateway deployment (Phase 3.3A)

Подготовка репозитория для развёртывания `dsgateway` на одной Oracle Cloud
Always Free VM. Первый публичный deployment — **только `-mock`**: без
DeepSeek API key, без трат, чтобы сначала доказать публичную связность.

```
Windows/Mac client (ds / iPhone DS Remote)
          |
          | HTTPS :443
          v
     Caddy (TLS, Let's Encrypt)
          |
          | localhost HTTP
          v
     127.0.0.1:8080
     dsgateway (-mock)
```

Публичные порты: `22` (SSH-администрирование), `80` (ACME/редирект),
`443` (Caddy HTTPS). **`8080` НЕ должен быть публично открыт.**

Это test-only инфраструктура: временный hostname `sslip.io`. Позже обычный
домен заменит его без изменения кода gateway.

---

## 1. OCI Console — создание VM

1. Compute → Instances → Create instance.
2. **Image:** Ubuntu (LTS, amd64 или arm64 по выбору shape).
3. **Shape:**
   - Primary: `VM.Standard.E2.1.Micro` (amd64).
   - Fallback: `VM.Standard.A1.Flex` (arm64/Ampere) — минимальная свободная
     аллокация. Oracle может ограничивать capacity — это ожидаемо.
4. SSH keys: добавить свой публичный ключ (deploy-скрипт использует
   соответствующий приватный ключ).
5. Create. Записать публичный IPv4.

### Ingress (Security List / Network Security Group)

| Порт | Source | Назначение |
|------|--------|-----------|
| TCP 22 | ваш admin IP (практично) | SSH-администрирование |
| TCP 80 | `0.0.0.0/0` | ACME (HTTP-01) / HTTPS redirect |
| TCP 443 | `0.0.0.0/0` | публичный HTTPS gateway |

**НЕ добавлять** `TCP 8080 from 0.0.0.0/0`.

Если включён Ubuntu host firewall (ufw), открыть 22/80/443:
```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### Определение архитектуры VM

```bash
uname -m
# x86_64  → используй --arch amd64
# aarch64 → используй --arch arm64
```

---

## 2. Сборка Linux-бинарников (на Mac)

```bash
make build-linux-amd64   # bin/dsgateway-linux-amd64
make build-linux-arm64   # bin/dsgateway-linux-arm64
make build-linux         # оба
```

Статические (`CGO_ENABLED=0`); на Oracle VM Go не нужен.

---

## 3. Деплой (с доверенного Mac)

```bash
./deploy/oci/deploy.sh \
  --ip 203.0.113.42 \
  --ssh-host 203.0.113.42 \
  --ssh-key ~/.ssh/oracle_vm_key \
  --arch amd64
```

Флаги: `--host` (готовый hostname, напр. `203-0-113-42.sslip.io`), `--ip`
(выводит `sslip.io` автоматически), `--ssh-host`, `--ssh-key`, `--arch`
(`amd64|arm64`), `--user` (default `ubuntu`), `--build`.

`deploy.sh` (с Mac): валидирует аргументы → собирает/выбирает бинарник →
`scp` файлов → удалённый `sudo bash install.sh` → проверяет публичный
`https://<hostname>/healthz` (первая выдача TLS может занять ~минуту).

Повторный запуск безопасен/idempotent: бинарник и конфиг обновляются,
пакеты/пользователь не дублируются.

### Вручную на VM (альтернатива)

```bash
sudo bash /tmp/dsremote-deploy/install.sh /tmp/dsremote-deploy/dsgateway 203-0-113-42.sslip.io
```

---

## 4. Файловая система VM

```
/opt/dsremote/
    dsgateway          # статический бинарник (owner: dsremote)
    run-gateway.sh     # launcher (mode mock|real)

/etc/dsremote/
    gateway.env        # root-owned, mode 0640
                       # DS_GATEWAY_MODE=mock
                       # DS_GATEWAY_LISTEN=127.0.0.1:8080

/etc/systemd/system/dsgateway.service
/etc/caddy/Caddyfile   # генерируется из Caddyfile.template
```

Пользователь: системный `dsremote` (nologin, без интерактивного входа).
Приложение — не в `/home/ubuntu`.

---

## 5. systemd

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now dsgateway
sudo systemctl status dsgateway
journalctl -u dsgateway
```

Unit: `User=dsremote`, `Restart=on-failure`, `RestartSec=3`,
`EnvironmentFile=/etc/dsremote/gateway.env`, умеренное hardening
(`NoNewPrivileges`, `PrivateTmp`, `ProtectHome`, `ProtectSystem=full`).
Журнал не содержит секретов (env не печатается; mock-режим не требует key).

---

## 6. HTTPS / Caddy

Caddy — TLS terminator с публично доверенным сертификатом (Let's Encrypt).
HTTP → HTTPS redirect автоматический. Streaming (SSE) сохраняется
(без buffering middleware). `reverse_proxy` только на `127.0.0.1:8080`.

Проверка:

```bash
curl -sS https://203-0-113-42.sslip.io/healthz
# {"status":"ok"}
```

---

## 7. Публичный mock E2E (acceptance test)

Mac-клиент:

```bash
./bin/ds --remote https://203-0-113-42.sslip.io
```

или Windows:

```powershell
.\ds.exe --remote https://203-0-113-42.sslip.io
```

Ожидание: `QR`, `Code: XXX XXX`, `Waiting for approval...`.

iPhone DS Remote Gateway URL: `https://203-0-113-42.sslip.io`
→ `Scan QR` → `Face ID` → `Approve`.

Терминал: `✓ Approved`, `DeepSeek V4 Flash · Thinking High`,
`DeepSeek ready.`; prompt `hello` → `mock reply to: hello`.
iPhone: `End Session`; следующий prompt в терминале: `not_found`.

### Mac-независимость

Локальный `dsgateway` на Mac должен быть **остановлен**. Клиент соединяется
только с `https://<hostname>`. Доказательство: `--remote` указывает на
публичный hostname; в `journalctl -u dsgateway` на Oracle видны сессии.

---

## 8. Security negative tests

1. `http://<hostname>/healthz` → redirect на HTTPS.
2. `https://<hostname>/healthz` → 200.
3. `http://<PUBLIC_IP>:8080/healthz` → **недоступен с интернета**.
4. random pairing code → rejected.
5. expired pairing → rejected.
6. wrong control token → rejected.
7. End Session → уничтожает сессию.

Без деструктивных сканеров.

---

## 9. Ограничения этого этапа

- Только mock-провайдер: никакого реального DeepSeek key на Oracle.
- `sslip.io` — test-only; позднее заменяется обычным доменом.
- iPhone НЕ передаёт API key (Phase 3.4 — следующий этап).
- Нет Terraform/OCI SDK: Security Lists настраиваются вручную в Console.
