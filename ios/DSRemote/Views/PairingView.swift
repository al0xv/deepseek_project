import AVFoundation
import SwiftUI
import UIKit

struct PairingView: View {
    @ObservedObject var viewModel: AppViewModel
    @State private var showScanner = false
    @State private var cameraDenied = false

    var body: some View {
        Form {
            Section(header: Text("Gateway")) {
                Text(viewModel.gatewayURLString.isEmpty ? "—" : viewModel.gatewayURLString)
                    .foregroundColor(.secondary)
            }
            if cameraDenied {
                Section {
                    Text("Camera access is disabled.")
                    Button("Open Settings") {
                        if let url = URL(string: UIApplication.openSettingsURLString) {
                            UIApplication.shared.open(url)
                        }
                    }
                }
            }
            Section(header: Text("Model")) {
                Picker("Model", selection: Binding(
                    get: { viewModel.selectedModel },
                    set: { viewModel.setModel($0) }
                )) {
                    ForEach(DeepSeekModel.allCases) { model in
                        Text(model.displayName).tag(model)
                    }
                }
                .pickerStyle(.menu)
            }
            Section(header: Text("Thinking")) {
                Picker("Thinking", selection: Binding(
                    get: { viewModel.selectedThinking },
                    set: { viewModel.setThinking($0) }
                )) {
                    ForEach(ThinkingMode.allCases) { mode in
                        Text(mode.displayName).tag(mode)
                    }
                }
                .pickerStyle(.menu)
            }
            Section {
                Button {
                    startScanning()
                } label: {
                    Label("Scan QR", systemImage: "qrcode.viewfinder")
                }
                .disabled(viewModel.isApproving)
            }
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
                .disabled(viewModel.isApproving)
            }
        }
        .sheet(isPresented: $showScanner) {
            QRScannerScreen(
                onQRScanned: { raw in
                    Task { await viewModel.handleScannedQR(raw) }
                },
                onPermissionDenied: {
                    cameraDenied = true
                }
            )
        }
    }

    private func startScanning() {
        viewModel.prepareForScan()
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized, .notDetermined:
            showScanner = true
        default:
            cameraDenied = true
        }
    }
}

