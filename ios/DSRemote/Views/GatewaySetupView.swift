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
            Section(header: Text("DeepSeek API Key")) {
                SecureField("sk-...", text: $viewModel.apiKey)
                Button("Save Key") { viewModel.saveAPIKey() }
                Button("Delete Key", role: .destructive) { viewModel.deleteAPIKey() }
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
