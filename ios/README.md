# iOS (MVP3) — placeholder

Native iOS-контроллер (SwiftUI) запланирован в MVP3. Реализация намеренно
отложена: она не может быть проверена в этом окружении, а протокол и core
уже готовы к подключению.

## План (Phase 3.1–3.4)

- `KeychainStore.swift` — хранение DeepSeek API key в iOS Keychain.
- `QRScannerView.swift` — скан QR (AVFoundation), парсинг
  `{"v","session_id","pairing_token","gateway_url"}`.
- `SessionController.swift` — Face ID (LocalAuthentication) → `POST /v1/pair`,
  показ Active Session, `POST /v1/sessions/{id}/close`.
- Gateway: TLS + приём `api_key` при approve (живёт только в RAM сессии).

## Требования к gateway для iOS

- HTTPS (ATS). Для dev — mkcert/самоподписанный сертификат с ATS-исключением.
- approve endpoint уже принимает `api_key` (поле в `ApproveRequest`).
