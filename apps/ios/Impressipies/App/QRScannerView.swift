import AVFoundation
import SwiftUI

/// Reads the server address from the QR code on the web app's App setup page.
enum ServerCode {
    /// The origin (scheme, host, port) of an http(s) URL, or nil for anything else.
    static func origin(from text: String) -> URL? {
        guard let url = URL(string: text.trimmingCharacters(in: .whitespacesAndNewlines)),
              let scheme = url.scheme?.lowercased(), scheme == "http" || scheme == "https",
              let host = url.host() else { return nil }
        var c = URLComponents()
        c.scheme = scheme
        c.host = host
        c.port = url.port
        return c.url
    }
}

/// Full-screen camera that reports the first server code it sees.
struct QRScannerSheet: View {
    let onScan: (URL) -> Void
    @Environment(\.dismiss) private var dismiss
    @State private var denied = false

    var body: some View {
        NavigationStack {
            ZStack {
                ScannerRepresentable(onCode: { text in
                    if let url = ServerCode.origin(from: text) { onScan(url); dismiss() }
                }, onDenied: { denied = true })
                .ignoresSafeArea()
                if denied {
                    VStack(spacing: 12) {
                        Text("Camera access is off").font(.headline)
                        Text("Allow the camera in Settings, or type the server address instead.")
                            .multilineTextAlignment(.center)
                        Button("Open Settings") {
                            if let url = URL(string: UIApplication.openSettingsURLString) { UIApplication.shared.open(url) }
                        }
                        .buttonStyle(PopButtonStyle())
                    }
                    .padding(24)
                    .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 20))
                    .padding(24)
                } else {
                    VStack {
                        Spacer()
                        Text("Point the camera at the code on Impressipes → App setup")
                            .font(.callout.weight(.semibold))
                            .multilineTextAlignment(.center)
                            .padding(12)
                            .background(.regularMaterial, in: Capsule())
                            .padding(.bottom, 32)
                            .padding(.horizontal, 20)
                    }
                }
            }
            .navigationTitle("Scan QR code")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } } }
        }
    }
}

private struct ScannerRepresentable: UIViewControllerRepresentable {
    let onCode: (String) -> Void
    let onDenied: () -> Void

    func makeUIViewController(context: Context) -> ScannerController {
        let c = ScannerController()
        c.onCode = onCode
        c.onDenied = onDenied
        return c
    }

    func updateUIViewController(_ controller: ScannerController, context: Context) {}
}

private final class ScannerController: UIViewController, AVCaptureMetadataOutputObjectsDelegate {
    var onCode: ((String) -> Void)?
    var onDenied: (() -> Void)?
    private let session = AVCaptureSession()
    private var preview: AVCaptureVideoPreviewLayer?
    private var done = false

    override func viewDidLoad() {
        super.viewDidLoad()
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized: configure()
        case .notDetermined:
            AVCaptureDevice.requestAccess(for: .video) { ok in
                DispatchQueue.main.async { ok ? self.configure() : self.onDenied?() }
            }
        default: onDenied?()
        }
    }

    private func configure() {
        guard let device = AVCaptureDevice.default(for: .video),
              let input = try? AVCaptureDeviceInput(device: device),
              session.canAddInput(input) else { return onDenied?() ?? () }
        session.addInput(input)
        let output = AVCaptureMetadataOutput()
        guard session.canAddOutput(output) else { return }
        session.addOutput(output)
        output.setMetadataObjectsDelegate(self, queue: .main)
        output.metadataObjectTypes = [.qr]
        let layer = AVCaptureVideoPreviewLayer(session: session)
        layer.videoGravity = .resizeAspectFill
        layer.frame = view.bounds
        view.layer.addSublayer(layer)
        preview = layer
        DispatchQueue.global(qos: .userInitiated).async { self.session.startRunning() }
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        preview?.frame = view.bounds
    }

    override func viewWillDisappear(_ animated: Bool) {
        super.viewWillDisappear(animated)
        if session.isRunning { session.stopRunning() }
    }

    func metadataOutput(_ output: AVCaptureMetadataOutput, didOutput objects: [AVMetadataObject], from connection: AVCaptureConnection) {
        guard !done, let text = (objects.first as? AVMetadataMachineReadableCodeObject)?.stringValue,
              ServerCode.origin(from: text) != nil else { return }
        done = true
        UINotificationFeedbackGenerator().notificationOccurred(.success)
        onCode?(text)
    }
}
