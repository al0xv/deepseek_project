# DS Remote — iOS Controller (Phase 3.1)

Native SwiftUI-контроллер для подтверждения/завершения terminal-сессий
DeepSeek через 6-digit pairing code. Фаза 3.1: **ручной ввод кода**, QR-скан
намеренно не реализован (Phase 3.2).

## Requirements

- macOS с Xcode 16+ (проверено на Xcode 26.4, Swift 6.3)
- iOS Deployment Target: **17.0**
- Никаких third-party зависимостей (чистые SwiftUI/Foundation/URLSession/
  LocalAuthentication/Security)

## Структура

```
ios/
  project.yml              # спецификация проекта (xcodegen)
  DSRemote.xcodeproj       # сгенерированный проект — открывать в Xcode
  Info.plist               # NSLocalNetworkUsageDescription + ATS (local networking)
  DSRemote/
    App/DSRemoteApp.swift
    Models/ControlSession.swift, AppError.swift
    Networking/GatewayClient.swift, GatewayURLValidator.swift, PairingCode.swift
    Security/BiometricAuthenticator.swift, KeychainStore.swift
    ViewModels/AppViewModel.swift
    Views/ContentView.swift, GatewaySetupView.swift, PairingView.swift, ActiveSessionView.swift
  DSRemoteTests/           # unit tests (16 кейсов)
```

## Как открыть и собрать

```bash
open ios/DSRemote.xcodeproj      # в Xcode
# или headless:
xcodebuild -project ios/DSRemote.xcodeproj -scheme DSRemote \
  -destination 'platform=iOS Simulator,name=iPhone 17' build
```

Unit tests:

```bash
xcodebuild test -project ios/DSRemote.xcodeproj -scheme DSRemote \
  -destination 'platform=iOS Simulator,name=iPhone 17'
```

Если `DSRemote.xcodeproj` нужно перегенерировать из `project.yml`:

```bash
brew install xcodegen
cd ios && xcodegen generate
```

## Local Network permission / ATS

- `NSLocalNetworkUsageDescription` задан: *"DS Remote connects to your local
  DeepSeek gateway to approve terminal sessions."* — iOS покажет системный
  диалог при первом подключении к LAN.
- ATS: только `NSAllowsLocalNetworking = true` — разрешает HTTP к локальной
  сети (LAN IP), **не** отключая ATS для публичного интернета. Никаких
  глобальных `NSAllowsArbitraryLoads`. Это самый узкий вариант для
  development LAN scenario. Production HTTPS — отдельная фаза.

## Настройка gateway URL

При первом запуске приложение показывает экран Setup:

```
Gateway
[ http://192.168.20.13:8080 ]

DeepSeek API Key
[ sk-... ]
[ Save Key ]  [ Delete Key ]

[ Continue ]
```

- Gateway URL сохраняется в `UserDefaults` (это не secret).
- Валидация: scheme `http`/`https`, host обязателен; иначе понятная ошибка.
- API key сохраняется **только** в iOS Keychain (`KeychainStore` через
  `SecItem*`). Никогда в UserDefaults, никогда в `print()`/логах.

> В Phase 3.1 ключ хранится и управляется, но в approve-запрос не
> передаётся: gateway работает с `-mock`, а session-scoped доставка key —
> Phase 3.3.

## Manual pairing flow

1. На Mac: `./bin/dsgateway -mock -listen 0.0.0.0:8080`.
2. В терминале (Mac/Windows): `./bin/ds --remote http://<MAC_IP>:8080` →
   терминал показывает `Code: XXX XXX` и `Waiting for approval...`.
3. В приложении: ввести код `XXX XXX` (или `XXXXXX`), нажать **Approve**.
4. Face ID (LocalAuthentication). Отмена/провал Face ID → approve-запрос
   НЕ отправляется.
5. Успех → `ACTIVE SESSION` (session id + state). Терминал: `✓ Approved`.
6. Чат в терминале работает (mock).
7. **End Session** → `DELETE /v1/sessions/{id}` с `Authorization:
   Bearer <control_token>` → gateway уничтожает сессию; терминал видит
   `Error: not_found: session not found` при следующем prompt.
8. control_token живёт только в памяти процесса приложения; после
   уничтожения сессии недействителен. Если app убит — capability теряется
   (допустимо для MVP; gateway сам уничтожит сессию по idle/exit).

## Известные ограничения (Phase 3.1)

- Нет QR-скана (Phase 3.2), нет chat UI на iPhone, нет push/background.
- Нет session-scoped DeepSeek key delivery (Phase 3.3) — approve работает
  с mock-провайдером.
- Терминал не получает активного уведомления об End Session (нет push):
  он видит завершение при следующем prompt (`not_found`).
- Для запуска на физическом iPhone нужно выбрать Team в Xcode (Signing &
  Capabilities); для симулятора подпись не требуется.
- Один контролируемый сессии одновременно (multisession — вне scope).

## Deployment target

Выбран **iOS 17.0** — современный, поддерживается установленным Xcode 26;
позволяет async/await LocalAuthentication (`evaluatePolicy`), SwiftUI
`ObservableObject`, Keychain.
