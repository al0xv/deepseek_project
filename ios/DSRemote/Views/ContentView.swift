import SwiftUI

struct ContentView: View {
    @ObservedObject var viewModel: AppViewModel

    var body: some View {
        NavigationStack {
            Group {
                switch viewModel.phase {
                case .setup:
                    GatewaySetupView(viewModel: viewModel)
                case .pairing:
                    PairingView(viewModel: viewModel)
                case .active(let session):
                    ActiveSessionView(viewModel: viewModel, session: session)
                }
            }
            .navigationTitle("DS Remote")
        }
    }
}
