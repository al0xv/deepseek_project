import SwiftUI

struct ActiveSessionView: View {
    @ObservedObject var viewModel: AppViewModel
    let session: ControlSession

    var body: some View {
        Form {
            Section(header: Text("ACTIVE SESSION")) {
                LabeledContent("Session", value: String(session.sessionID.prefix(12)))
                LabeledContent("State", value: session.state.rawValue)
            }
            if let message = viewModel.errorMessage {
                Section { Text(message).foregroundColor(.red) }
            }
            Section {
                Button("End Session", role: .destructive) {
                    Task { await viewModel.endSession() }
                }
            }
        }
    }
}
