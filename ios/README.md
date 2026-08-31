# DS Remote — iOS Controller (Phase 3.2)

Native SwiftUI-контроллер для подтверждения/завершения terminal-сессий
DeepSeek. Pairing: **QR-скан** (основной) + **ручной ввод 6-digit code**
(fallback). QR-скан реализован на системном AVFoundation.

## Requirements

- macOS с Xcode 16+ (проверено на Xcode 26.4, Swift 6.3)
- iOS Deployment Target: **17.0**
- Никаких third-party зависимостей (чистые SwiftUI/Foundation/URLSession/
  LocalAuthentication/Security/AVFoundation)

## Verification status

| Check | Status |
|-------|--------|
| iOS build + unit tests (simulator) | PASS (автоматически) |
| Real iPhone QR verification | **PASS** (физический iPhone) |
| Real iPhone Face ID verification | **PASS** (физический iPhone) |
| Real iPhone approval flow | **PASS** (физический iPhone) |
| Real iPhone End Session | **PASS** (физический iPhone) |
| Real DeepSeek API/streaming/multi-turn | **PASS** (реальный платный запрос) |
| Real Windows verification | `AWAITING REAL WINDOWS VERIFICATION` |

### Fixed crash: missing NSFaceIDUsageDescription

При первом физическом тесте app падала сразу после скана QR:
«DSRemote crashed because it attempted to access privacy sensitive data
without a usage description. Add NSFaceIDUsageDescription». Временный фикс
в Xcode перенесён в source of truth — `ios/Info.plist`:

```
NSFaceIDUsageDescription = "DS Remote uses Face ID to approve temporary terminal sessions."
```

Ключ переживает `xcodegen generate` (Info.plist не генерируется xcodegen —
он ссылается на файл через `INFOPLIST_FILE`). Регрессионная проверка:
`make ios-plist-check` (`scripts/check-ios-plist.sh`) убеждается, что в
Info.plist есть `NSCameraUsageDescription`, `NSLocalNetworkUsageDescription`,
`NSFaceIDUsageDescription` и нет `NSMicrophoneUsageDescription` /
`NSPhotoLibraryUsageDescription`.

> Примечание: личный `DEVELOPMENT_TEAM` пользователя в репозиторий не
> коммитится; при сборке на физическом устройстве его нужно выбрать в Xcode
> (Signing & Capabilities).

## Структура

```
ios/
  project.yml              # спецификация проекта (xcodegen)
  DSRemote.xcodeproj       # сгенерированный проект — открывать в Xcode
  Info.plist               # NSLocalNetworkUsageDescription + ATS (local networking)
  DSRemote/
    App/DSRemoteApp.swift
    Models/ControlSession.swift, AppError.swift
    Networking/GatewayClient.swift, GatewayURLValidator.swift, PairingCode.swift,
             QRPairingParser.swift
    Security/BiometricAuthenticator.swift, KeychainStore.swift
    ViewModels/AppViewModel.swift
    Views/ContentView.swift, GatewaySetupView.swift, PairingView.swift,
         ActiveSessionView.swift, QRScannerView.swift, QRScannerScreen.swift
  DSRemoteTests/           # unit tests (36 кейсов)
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

> **Stub (Phase 3.4):** ключ хранится и управляется только в Keychain, но в
> approve-запрос **не передаётся** (gateway отклоняет `api_key` как
> `not_implemented`, `501`). Рабочий способ сегодня: `DEEPSEEK_API_KEY` на
> gateway (или `-mock`).

## Model & Thinking selection

На экране Pairing (до Scan QR) доступны селекторы:

```
Model     [ V4 Flash ▾ ]   (V4 Flash / V4 Pro)
Thinking  [ High ▾ ]       (Off / Low / High / Max)
```

- Default: **V4 Flash / High** (продуктовый default).
- Выбор per-session: передаётся в approve-запросе (`model`, `thinking`,
  `reasoning_effort`) и живёт в RAM сессии; immutable после approval.
- Preferences сохраняются в `UserDefaults` (не secret; API key — только в
  Keychain).
- Терминал показывает canonical значения, подтверждённые gateway, напр.
  `DeepSeek V4 Flash · Thinking High`.
- Нет `Medium` в UI; значения `low|high|max` — только для thinking enabled.
- QR остаётся `dsremote://pair?v=1&code=XXXXXX` (без настроек).

## Manual pairing flow (fallback)

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

## QR pairing flow

Основной способ pairing: `Scan QR` → камера → Face ID → approve.

1. На экране Pairing нажать **Scan QR** (кнопка `qrcode.viewfinder`).
2. Разрешить доступ к камере (системный диалог; `NSCameraUsageDescription`
   в Info.plist).
3. Навести камеру на QR в терминале.
4. При успешном распознавании валидного payload: scanner останавливается,
   запускается Face ID, затем существующий approve flow.
5. Успех → `ACTIVE SESSION`. Терминал: `✓ Approved`.

### Payload format

QR-код, генерируемый gateway, содержит минимальный URI:

```
dsremote://pair?v=1&code=472913
```

- Никаких secrets: нет API key, control_token, session id, gateway URL.
- Парсинг строгий: чужие QR (`https://...`, другие scheme/host, не-6-значный
  code, неизвестная версия, лишние параметры) отклоняются с
  `Not a DS Remote pairing QR`, без открытия содержимого и без approval.
- Gateway URL приложение берёт из настроек (Phase 3.1), в QR его нет.

### Duplicate frames

После первого валидного QR сканер блокируется (`didCaptureValidQR`),
плюс AppViewModel дополнительно не повторяет одинаковый raw payload —
даже если камера видит один QR много кадров, Face ID/approve происходят
один раз.

### Biometric cancellation

Отмена Face ID после скана → approve не отправляется, сессия остаётся
`WAITING`, приложение возвращается на pairing screen. Повторный `Scan QR`
с тем же QR разрешён (`prepareForScan` сбрасывает guard).

### Invalid / expired / camera states

- Чужой QR → feedback `Not a DS Remote pairing QR`, сканер продолжает работать.
- Pairing expired → существующая серверная ошибка отображается как
  `Pairing expired`; активной сессии нет.
- Camera denied → `Camera access is disabled.` + **Open Settings**; ручной
  ввод кода остаётся доступен.
- Камера недоступна (например simulator) → `Camera unavailable on this
  device.`; ручной ввод кода остаётся доступен.

## Camera permission

- `NSCameraUsageDescription`: *"DS Remote uses the camera to scan temporary
  terminal pairing QR codes."*
- Не запрашиваются: микрофон, фото, контакты, геолокация (в Info.plist нет
  соответствующих ключей).
- Обрабатываются `.authorized`, `.notDetermined`, `.denied`, `.restricted`.

## Известные ограничения (Phase 3.2)

- Нет chat UI на iPhone, нет push/background, нет session-scoped DeepSeek key
  delivery (Phase 3.4 — stub) — approve работает с ключом gateway или
  mock-провайдером.
- Терминал не получает активного уведомления об End Session (нет push):
  он видит завершение при следующем prompt (`not_found`).
- Для запуска на физическом iPhone нужно выбрать Team в Xcode (Signing &
  Capabilities); для симулятора подпись не требуется.
- Один контролируемый сессии одновременно (multisession — вне scope).
- Simulator: физического сканирования камерой нет (только Mac-камера при
  наличии); результат сканирования подтверждается только на реальном iPhone.

## Physical iPhone test procedure

1. `make build`, получить LAN IP: `ipconfig getifaddr en0`.
2. Mac: `./bin/dsgateway -mock -listen 0.0.0.0:8080`.
3. Terminal: `./bin/ds --remote http://<MAC_IP>:8080`.
4. iPhone: DS Remote → Gateway URL = `http://<MAC_IP>:8080` → **Scan QR** →
   выдать camera permission → навести на QR.
5. Ожидание: `QR recognized → Face ID → ACTIVE SESSION`; терминал: `✓ Approved`.
6. `You > hello world` → `mock reply to: hello world`.
7. **End Session** → сессия уничтожена.

Негативные тесты: чужой QR (никакого Face ID), Face ID cancel (нет approve),
QR после expiry (серверная ошибка), camera denied (fallback на код).

## Deployment target

Выбран **iOS 17.0** — современный, поддерживается установленным Xcode 26;
позволяет async/await LocalAuthentication (`evaluatePolicy`), SwiftUI
`ObservableObject`, Keychain.
