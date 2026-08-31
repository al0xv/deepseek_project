import SwiftUI

struct GatewaySetupView: View {
    @ObservedObject var viewModel: AppViewModel

    var body: some View {
        Form {
            Section(header: Text("Gateway")) {
                TextField("http://192.168.1.42:8080", text: $viewModel.gatewayURLString)
                    .keyboardType(.URL)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
            }
            // STUB (Phase 3.4): the API key is stored in the Keychain for a
            // future secure-delivery feature, but it is NOT sent with approve
            // today. The gateway rejects api_key as "not_implemented" and uses
            // its own DEEPSEEK_API_KEY (or the mock provider) instead.
            Section(header: Text("DeepSeek API Key")) {
                SecureField("sk-...", text: $viewModel.apiKey)
                Button("Save Key") { viewModel.saveAPIKey() }
                Button("Delete Key", role: .destructive) { viewModel.deleteAPIKey() }
                Text("Stub (Phase 3.4): stored locally in the iOS Keychain only — not sent to the gateway yet.")
                    .font(.footnote)
                    .foregroundColor(.secondary)
            }
            if let message = viewModel.errorMessage {
                Section { Text(message).foregroundColor(.red) }
            }
            Section {
                Button("Continue") { viewModel.saveGatewayURL() }
            }
        }
    }
}
