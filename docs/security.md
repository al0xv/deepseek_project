# Security Model

## Инвариант

**DeepSeek API key никогда не покидает доверенное устройство** (env на Mac
в MVP0–2 / iPhone Keychain в MVP3) и не попадает в RAM чужого ПК, его shell
history, логи или файлы.

## Threats и митигации

| Угроза | Статус | Митигация |
|--------|--------|-----------|
| API key на чужом ПК | предотвращено | ключ только в `dsgateway` (env) / RAM gateway после approve (MVP3); `ds` не содержит кода чтения ключа |
| Shell history чужого ПК | предотвращено | клиент принимает ноль секретных аргументов; команда всегда просто `ds` |
| Malware/keylogger/screen-capture на чужом ПК | **непредотвратимо для содержимого диалога** (оно на экране и в RAM процесса) | принято как компромисс; но API key защищён |
| Украденный pairing QR / code | ограничено | token 32 байта (43 символа base64url), single-use, 120s; код 6 цифр + 120s + single-use + rate limit на approve не нужен (случайность + короткое окно) |
| Перехват network traffic | MVP0-2: localhost/dev; MVP3: TLS | в MVP1–2 трафик только на localhost либо явно insecure LAN для разработки; в MVP3 — HTTPS (требование iOS ATS) |
| Leaked logs | предотвращено | gateway не логирует key, prompts, токены, ответы; только факты `session created/destroyed` |
| Malicious client / DoS | ограничено | `maxSessions=8`, rate limit 20 create/мин, таймауты (pair 120s, idle 5m, gen 60s), размер prompt ≤64KB |
| Session hijacking | ограничено | session_id (16B) и pairing_token (32B) случайные, single-use, никогда не переиспользуются |
| Возобновление сессии после destroy | предотвращено | объект удаляется, история/ключ обнуляются, resume/reconnect не поддерживается |

## Что мы сознательно НЕ решаем

- Защиту от keylogger на чужом ПК: содержимое prompts/answers читаемо злоумышленником.
- Доверие к malicious terminal: терминал может подменить вывод; это его риск,
  не утечка секретов владельца.

## Усиления для MVP3 (задокументированы, не реализованы)

- approve, подписанный ключом iPhone (чтобы чужой ПК с украденным QR не мог
  одобрить сам);
- mutual TLS / привязка сертификатов;
- доставка API key с iPhone в gateway только по HTTPS, существование только в
  RAM активной сессии, затирание при destroy.
