import Foundation

/// The subset of the gateway protocol the controller needs.
protocol GatewayClientProtocol {
    func approve(code: String, gatewayURL: URL, settings: GenerationSettings) async throws -> ControlSession
    func endSession(sessionID: String, controlToken: String, gatewayURL: URL) async throws
}

/// Production HTTP client backed by URLSession. No third-party networking.
struct GatewayClient: GatewayClientProtocol {
    let session: URLSession

    init(session: URLSession = .shared) {
        self.session = session
    }

    func approve(code: String, gatewayURL: URL, settings: GenerationSettings) async throws -> ControlSession {
        guard let normalized = PairingCode.normalize(code) else {
            throw AppError.invalidPairingCode
        }
        var payload: [String: Any] = ["pairing_code": normalized]
        payload["model"] = settings.model.rawValue
        payload["thinking"] = settings.thinking.thinkingEnabled
        if let effort = settings.thinking.reasoningEffort {
            payload["reasoning_effort"] = effort
        }

        var request = URLRequest(url: Self.url(gatewayURL, path: "/v1/pair"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: payload)

        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw AppError.gatewayUnavailable }
        guard http.statusCode == 200 else {
            throw Self.error(from: data, status: http.statusCode)
        }
        return try JSONDecoder().decode(ControlSession.self, from: data)
    }

    func endSession(sessionID: String, controlToken: String, gatewayURL: URL) async throws {
        var request = URLRequest(url: Self.url(gatewayURL, path: "/v1/sessions/\(sessionID)"))
        request.httpMethod = "DELETE"
        request.setValue("Bearer \(controlToken)", forHTTPHeaderField: "Authorization")

        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw AppError.gatewayUnavailable }
        guard http.statusCode == 204 else {
            throw Self.error(from: data, status: http.statusCode)
        }
    }

    // MARK: - Helpers

    private static func url(_ base: URL, path: String) -> URL {
        URL(string: base.absoluteString + path) ?? base
    }

    private static func error(from data: Data, status: Int) -> AppError {
        struct Body: Decodable { let code: String; let message: String }
        if let body = try? JSONDecoder().decode(Body.self, from: data) {
            switch body.code {
            case "pairing_expired": return .pairingExpired
            case "already_approved": return .alreadyApproved
            case "not_found": return .sessionNotFound
            case "unauthorized": return .endSessionFailed
            default: return .server(code: body.code, message: body.message)
            }
        }
        return .gatewayUnavailable
    }
}
