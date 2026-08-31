import Foundation

/// DeepSeek models available for a session.
enum DeepSeekModel: String, CaseIterable, Identifiable {
    case flash = "deepseek-v4-flash"
    case pro = "deepseek-v4-pro"

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .flash: return "V4 Flash"
        case .pro: return "V4 Pro"
        }
    }
}

/// Thinking mode for a session. "Off" maps to thinking disabled; the rest map
/// to the DeepSeek `reasoning_effort` values.
enum ThinkingMode: String, CaseIterable, Identifiable {
    case off
    case low
    case high
    case max

    var id: String { displayName }

    var displayName: String {
        switch self {
        case .off: return "Off"
        case .low: return "Low"
        case .high: return "High"
        case .max: return "Max"
        }
    }

    var thinkingEnabled: Bool { self != .off }

    /// `reasoning_effort` API value; nil when thinking is disabled.
    var reasoningEffort: String? {
        switch self {
        case .off: return nil
        case .low: return "low"
        case .high: return "high"
        case .max: return "max"
        }
    }
}

/// Per-session generation settings selected before approval.
/// Display names are never sent as API values; rawValue/effort are.
struct GenerationSettings: Equatable {
    var model: DeepSeekModel
    var thinking: ThinkingMode

    static let `default` = GenerationSettings(model: .flash, thinking: .high)
}
