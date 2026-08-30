import Foundation

/// User-facing errors. Messages never contain secrets (API key / control token).
enum AppError: Error, LocalizedError, Equatable {
    case invalidGatewayURL
    case invalidPairingCode
    case pairingExpired
    case alreadyApproved
    case gatewayUnavailable
    case sessionNotFound
    case endSessionFailed
    case biometricCancelled
    case biometricFailed
    case notDSRemoteQR
    case server(code: String, message: String)

    var errorDescription: String? {
        switch self {
        case .invalidGatewayURL:
            return "Invalid gateway URL. Use http(s)://host:port"
        case .invalidPairingCode:
            return "Enter a 6-digit pairing code"
        case .pairingExpired:
            return "Pairing expired. Create a new terminal session."
        case .alreadyApproved:
            return "Session already approved"
        case .gatewayUnavailable:
            return "Gateway unavailable. Check the URL and that the gateway is running."
        case .sessionNotFound:
            return "Session no longer exists"
        case .endSessionFailed:
            return "Failed to end the session"
        case .biometricCancelled:
            return "Biometric authentication cancelled"
        case .biometricFailed:
            return "Biometric authentication failed"
        case .notDSRemoteQR:
            return "Not a DS Remote pairing QR"
        case .server(_, let message):
            return message
        }
    }
}
