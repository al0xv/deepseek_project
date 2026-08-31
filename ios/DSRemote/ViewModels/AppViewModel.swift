import Foundation

@MainActor
final class AppViewModel: ObservableObject {
    enum Phase: Equatable {
        case setup
        case pairing
        case active(ControlSession)
    }

    @Published var phase: Phase
    @Published var gatewayURLString: String
    @Published var pairingCode: String = ""
    @Published var apiKey: String = ""
    @Published var errorMessage: String?
    @Published var isApproving = false
    @Published var selectedModel: DeepSeekModel
    @Published var selectedThinking: ThinkingMode

    private let client: GatewayClientProtocol
    private let biometrics: BiometricAuthenticating
    private let keychain: KeychainStoring
    private let defaults: UserDefaults
    /// Guards against re-approving the same scanned QR (duplicate frames).
    private var lastQRProcessed: String?

    private static let modelPrefKey = "selectedModel"
    private static let thinkingPrefKey = "selectedThinking"

    init(client: GatewayClientProtocol = GatewayClient(),
         biometrics: BiometricAuthenticating = BiometricAuthenticator(),
         keychain: KeychainStoring = KeychainStore(),
         defaults: UserDefaults = .standard) {
        self.client = client
        self.biometrics = biometrics
        self.keychain = keychain
        self.defaults = defaults
        let savedURL = defaults.string(forKey: "gatewayURL") ?? ""
        self.gatewayURLString = savedURL
        self.apiKey = keychain.loadAPIKey() ?? ""
        let savedModel = defaults.string(forKey: Self.modelPrefKey).flatMap(DeepSeekModel.init(rawValue:)) ?? .flash
        let savedThinking = defaults.string(forKey: Self.thinkingPrefKey).flatMap(ThinkingMode.init(rawValue:)) ?? .high
        self._selectedModel = Published(initialValue: savedModel)
        self._selectedThinking = Published(initialValue: savedThinking)
        self._phase = Published(initialValue: GatewayURLValidator.parse(savedURL) != nil ? .pairing : .setup)
    }

    var gatewayURL: URL? {
        GatewayURLValidator.parse(gatewayURLString)
    }

    /// Called before opening the scanner: clears the duplicate-QR guard so a
    /// cancelled approval can be retried by re-scanning the same QR.
    func prepareForScan() {
        lastQRProcessed = nil
        errorMessage = nil
    }

    // MARK: - Generation settings (persisted in UserDefaults, not secrets)

    func setModel(_ model: DeepSeekModel) {
        selectedModel = model
        defaults.set(model.rawValue, forKey: Self.modelPrefKey)
    }

    func setThinking(_ thinking: ThinkingMode) {
        selectedThinking = thinking
        defaults.set(thinking.rawValue, forKey: Self.thinkingPrefKey)
    }

    // MARK: - Gateway configuration

    func saveGatewayURL() {
        guard let url = gatewayURL else {
            errorMessage = AppError.invalidGatewayURL.errorDescription
            return
        }
        defaults.set(url.absoluteString, forKey: "gatewayURL")
        errorMessage = nil
        phase = .pairing
    }

    // MARK: - API key (Keychain only)

    // STUB (Phase 3.4): the key is persisted in the Keychain for a future
    // secure-delivery feature. It is deliberately NOT sent with the approve
    // request today; the gateway rejects api_key with "not_implemented".
    func saveAPIKey() {
        let trimmed = apiKey.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            errorMessage = "Enter an API key"
            return
        }
        do {
            try keychain.saveAPIKey(trimmed)
            errorMessage = nil
        } catch {
            errorMessage = "Could not save API key"
        }
    }

    func deleteAPIKey() {
        try? keychain.deleteAPIKey()
        apiKey = ""
        errorMessage = nil
    }

    // MARK: - Pairing approval

    /// Handles a QR payload delivered by the scanner. Parsing is strict: an
    /// unrelated QR never reaches biometrics or the network.
    func handleScannedQR(_ rawValue: String) async {
        guard rawValue != lastQRProcessed else { return }
        lastQRProcessed = rawValue
        do {
            let payload = try QRPairingParser.parse(rawValue)
            pairingCode = payload.pairingCode
            await approve()
        } catch {
            errorMessage = Self.message(for: error)
        }
    }

    func approve() async {
        guard !isApproving else { return }
        isApproving = true
        defer { isApproving = false }
        guard gatewayURL != nil else {
            errorMessage = AppError.invalidGatewayURL.errorDescription
            return
        }
        guard PairingCode.normalize(pairingCode) != nil else {
            errorMessage = AppError.invalidPairingCode.errorDescription
            return
        }
        // No network mutation happens unless biometrics succeed.
        do {
            try await biometrics.authenticate(reason: "Approve this DeepSeek terminal session")
        } catch {
            errorMessage = Self.message(for: error)
            return
        }
        guard let url = gatewayURL else { return }
        do {
            let session = try await client.approve(
                code: pairingCode,
                gatewayURL: url,
                settings: GenerationSettings(model: selectedModel, thinking: selectedThinking)
            )
            phase = .active(session)
            pairingCode = ""
            errorMessage = nil
        } catch {
            errorMessage = Self.message(for: error)
        }
    }

    // MARK: - End session

    func endSession() async {
        guard let url = gatewayURL, case let .active(session) = phase else { return }
        do {
            try await client.endSession(sessionID: session.sessionID,
                                        controlToken: session.controlToken,
                                        gatewayURL: url)
            phase = .pairing
            errorMessage = nil
        } catch AppError.sessionNotFound {
            // The terminal already ended it; treat as done.
            phase = .pairing
            errorMessage = AppError.sessionNotFound.errorDescription
        } catch {
            errorMessage = Self.message(for: error)
            // active session is retained on failure.
        }
    }

    // MARK: - Error mapping

    private static func message(for error: Error) -> String {
        (error as? LocalizedError)?.errorDescription ?? "Something went wrong"
    }
}
