import Foundation

enum PairingCode {
    /// Normalizes user input to a 6-digit code.
    /// "472 913" and "472913" both produce "472913".
    /// Returns nil for anything that is not exactly six digits.
    static func normalize(_ input: String) -> String? {
        let digits = input.filter(\.isNumber)
        guard digits.count == 6 else { return nil }
        return digits
    }
}
