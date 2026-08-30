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

    private let client: GatewayClientProtocol
    private let biometrics: BiometricAuthenticating
    private let keychain: KeychainStoring
    private let defaults: UserDefaults

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
        self._phase = Published(initialValue: GatewayURLValidator.parse(savedURL) != nil ? .pairing : .setup)
    }

    var gatewayURL: URL? {
        GatewayURLValidator.parse(gatewayURLString)
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

    func approve() async {
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
            let session = try await client.approve(code: pairingCode, gatewayURL: url)
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
