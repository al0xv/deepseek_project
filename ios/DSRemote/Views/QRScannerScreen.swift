import SwiftUI

/// Full-screen scanner sheet: camera preview + Cancel + safe feedback for
/// invalid/unavailable states. On a valid QR it dismisses and forwards the
/// raw payload to the caller, which runs the existing approval flow.
struct QRScannerScreen: View {
    let onQRScanned: (String) -> Void
    let onPermissionDenied: () -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var feedbackMessage: String?

    var body: some View {
        NavigationStack {
            ZStack {
                Color.black.ignoresSafeArea()
                QRScannerView(
                    onQRScanned: { raw in
                        dismiss()
                        onQRScanned(raw)
                    },
                    onInvalidQR: {
                        feedbackMessage = "Not a DS Remote pairing QR"
                    },
                    onCameraUnavailable: {
                        feedbackMessage = "Camera unavailable on this device."
                    },
                    onPermissionDenied: {
                        dismiss()
                        onPermissionDenied()
                    }
                )
                .ignoresSafeArea()

                if let feedbackMessage {
                    VStack {
                        Spacer()
                        Text(feedbackMessage)
                            .font(.subheadline)
                            .foregroundColor(.white)
                            .padding(12)
                            .background(Color.black.opacity(0.75), in: Capsule())
                            .padding(.bottom, 48)
                    }
                }
            }
            .navigationTitle("Scan QR")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
        }
    }
}
