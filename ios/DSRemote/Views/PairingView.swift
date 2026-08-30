import SwiftUI

struct PairingView: View {
    @ObservedObject var viewModel: AppViewModel

    var body: some View {
        Form {
            Section(header: Text("Pairing Code")) {
                TextField("472 913", text: $viewModel.pairingCode)
                    .keyboardType(.numberPad)
                    .textInputAutocapitalization(.never)
            }
            if let message = viewModel.errorMessage {
                Section { Text(message).foregroundColor(.red) }
            }
            Section {
                Button("Approve") {
                    Task { await viewModel.approve() }
                }
            }
        }
    }
}
