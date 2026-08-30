import Foundation

/// Mirror of the gateway session state machine.
enum SessionState: String, Codable {
    case waiting = "WAITING"
    case approved = "APPROVED"
    case active = "ACTIVE"
    case destroyed = "DESTROYED"
}

/// A session controlled by this app after a successful pairing approval.
/// `controlToken` is a memory-only controller capability: it must never be
/// persisted, logged or displayed.
struct ControlSession: Codable, Equatable {
    let sessionID: String
    let state: SessionState
    let controlToken: String

    enum CodingKeys: String, CodingKey {
        case sessionID = "session_id"
        case state
        case controlToken = "control_token"
    }
}
