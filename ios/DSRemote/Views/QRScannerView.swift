import AVFoundation
import SwiftUI
import UIKit

/// Minimal AVFoundation QR scanner wrapped for SwiftUI.
///
/// The camera/AVCaptureSession lifecycle lives in `QRScannerViewController`;
/// this type only forwards parsed results and status events.
struct QRScannerView: UIViewControllerRepresentable {
    let onQRScanned: (String) -> Void
    let onInvalidQR: () -> Void
    let onCameraUnavailable: () -> Void
    let onPermissionDenied: () -> Void

    func makeCoordinator() -> Coordinator {
        Coordinator(onQRScanned: onQRScanned,
                    onInvalidQR: onInvalidQR,
                    onCameraUnavailable: onCameraUnavailable,
                    onPermissionDenied: onPermissionDenied)
    }

    func makeUIViewController(context: Context) -> UIViewController {
        QRScannerViewController(coordinator: context.coordinator)
    }

    func updateUIViewController(_ uiViewController: UIViewController, context: Context) {}

    final class Coordinator: NSObject, AVCaptureMetadataOutputObjectsDelegate {
        let onQRScanned: (String) -> Void
        let onInvalidQR: () -> Void
        let onCameraUnavailable: () -> Void
        let onPermissionDenied: () -> Void

        /// Stops further callbacks after the first valid QR so duplicate
        /// camera frames cannot trigger multiple approval flows.
        private var didCaptureValidQR = false

        init(onQRScanned: @escaping (String) -> Void,
             onInvalidQR: @escaping () -> Void,
             onCameraUnavailable: @escaping () -> Void,
             onPermissionDenied: @escaping () -> Void) {
            self.onQRScanned = onQRScanned
            self.onInvalidQR = onInvalidQR
            self.onCameraUnavailable = onCameraUnavailable
            self.onPermissionDenied = onPermissionDenied
        }

        func metadataOutput(_ output: AVCaptureMetadataOutput,
                            didOutput metadataObjects: [AVMetadataObject],
                            from connection: AVCaptureConnection) {
            guard !didCaptureValidQR,
                  let object = metadataObjects.first as? AVMetadataMachineReadableCodeObject,
                  let raw = object.stringValue else { return }
            // Strict parse: an unrelated QR only triggers feedback, never an
            // approval flow, and never opens the payload.
            if (try? QRPairingParser.parse(raw)) != nil {
                didCaptureValidQR = true
                DispatchQueue.main.async { self.onQRScanned(raw) }
            } else {
                DispatchQueue.main.async { self.onInvalidQR() }
            }
        }
    }
}


/// Owns the AVCaptureSession. Configuration/start/stop run on a serial queue
/// so the main thread is never blocked. Stops the session when the app
/// backgrounds or the view disappears.
final class QRScannerViewController: UIViewController {
    private let session = AVCaptureSession()
    private let sessionQueue = DispatchQueue(label: "com.deepseek.dsremote.qr.session")
    private let coordinator: QRScannerView.Coordinator
    private let previewLayer = AVCaptureVideoPreviewLayer()
    private var sessionConfigured = false

    init(coordinator: QRScannerView.Coordinator) {
        self.coordinator = coordinator
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not supported") }

    deinit {
        NotificationCenter.default.removeObserver(self)
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .black
        previewLayer.session = session
        previewLayer.videoGravity = .resizeAspectFill
        view.layer.addSublayer(previewLayer)
        observeAppLifecycle()
        checkPermission()
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        previewLayer.frame = view.bounds
    }

    override func viewWillDisappear(_ animated: Bool) {
        super.viewWillDisappear(animated)
        stopSession()
    }

    // MARK: - Permission

    private func checkPermission() {
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized:
            startSession()
        case .notDetermined:
            AVCaptureDevice.requestAccess(for: .video) { granted in
                DispatchQueue.main.async {
                    if granted {
                        self.startSession()
                    } else {
                        self.coordinator.onPermissionDenied()
                    }
                }
            }
        default:
            coordinator.onPermissionDenied()
        }
    }

    // MARK: - Session lifecycle

    private func startSession() {
        sessionQueue.async { [weak self] in
            guard let self, !self.session.isRunning else { return }
            if !self.sessionConfigured {
                guard let device = AVCaptureDevice.default(for: .video),
                      let input = try? AVCaptureDeviceInput(device: device) else {
                    DispatchQueue.main.async { self.coordinator.onCameraUnavailable() }
                    return
                }
                self.session.beginConfiguration()
                if self.session.canAddInput(input) {
                    self.session.addInput(input)
                }
                let output = AVCaptureMetadataOutput()
                if self.session.canAddOutput(output) {
                    self.session.addOutput(output)
                    output.setMetadataObjectsDelegate(self.coordinator, queue: self.sessionQueue)
                    output.metadataObjectTypes = [.qr]
                }
                self.session.commitConfiguration()
                self.sessionConfigured = true
            }
            self.session.startRunning()
        }
    }

    private func stopSession() {
        sessionQueue.async { [weak self] in
            guard let self, self.session.isRunning else { return }
            self.session.stopRunning()
        }
    }

    private func observeAppLifecycle() {
        NotificationCenter.default.addObserver(
            forName: UIApplication.willResignActiveNotification, object: nil, queue: .main
        ) { [weak self] _ in
            self?.stopSession()
        }
        NotificationCenter.default.addObserver(
            forName: UIApplication.didBecomeActiveNotification, object: nil, queue: .main
        ) { [weak self] _ in
            guard let self,
                  self.view.window != nil,
                  AVCaptureDevice.authorizationStatus(for: .video) == .authorized else { return }
            self.startSession()
        }
    }
}
