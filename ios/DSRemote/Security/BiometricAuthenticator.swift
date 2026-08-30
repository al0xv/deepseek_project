import Foundation
import LocalAuthentication

protocol BiometricAuthenticating {
    func authenticate(reason: String) async throws
}

/// Production implementation backed by LocalAuthentication (Face ID / Touch ID).
struct BiometricAuthenticator: BiometricAuthenticating {
    func authenticate(reason: String) async throws {
        let context = LAContext()
        var error: NSError?
        guard context.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &error) else {
            throw AppError.biometricFailed
        }
        do {
            let success = try await context.evaluatePolicy(
                .deviceOwnerAuthenticationWithBiometrics,
                localizedReason: reason
            )
            guard success else { throw AppError.biometricFailed }
        } catch let laError as LAError {
            if laError.code == .userCancel
                || laError.code == .appCancel
                || laError.code == .systemCancel {
                throw AppError.biometricCancelled
            }
            throw AppError.biometricFailed
        }
    }
}
