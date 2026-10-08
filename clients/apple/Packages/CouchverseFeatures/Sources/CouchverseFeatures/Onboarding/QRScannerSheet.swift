#if os(iOS)
    import CouchverseDesign
    import SwiftUI
    import VisionKit

    /// Scans a Couchverse QR code with the camera: a web "Connect a device" code, a TV's pairing
    /// code or a host's couch. Other codes are pointed out and scanning goes on.
    struct QRScannerSheet: View {
        /// The kind of link this scan is for; anything else is pointed out as not a Couchverse code.
        let expecting: DeepLink
        let onScan: (String) -> Void

        @Environment(\.dismiss) private var dismiss
        @State private var rejected = false

        var body: some View {
            NavigationStack {
                ZStack(alignment: .bottom) {
                    if DataScannerViewController.isSupported && DataScannerViewController.isAvailable {
                        QRScanner { payload in
                            if DeepLink(payload) == expecting {
                                onScan(payload)
                            } else {
                                rejected = true
                            }
                        }
                        .ignoresSafeArea()
                    } else {
                        ContentUnavailableView(
                            L10n.scannerTitle, systemImage: "camera", description: Text(L10n.scannerUnavailable))
                    }
                    Text(rejected ? L10n.scannerNotCouchverse : L10n.scannerHint)
                        .typeRole(Tokens.TypeRamp.body)
                        .padding(Tokens.Spacing.lg)
                        .glassEffect(.regular, in: Capsule())
                        .padding(Tokens.Spacing.xl)
                }
                .navigationTitle(L10n.scannerTitle)
                .navigationBarTitleDisplayMode(.inline)
                .toolbar {
                    ToolbarItem(placement: .cancellationAction) {
                        Button(L10n.commonCancel) { dismiss() }
                    }
                }
            }
        }
    }

    private struct QRScanner: UIViewControllerRepresentable {
        let onPayload: (String) -> Void

        func makeUIViewController(context: Context) -> DataScannerViewController {
            let scanner = DataScannerViewController(
                recognizedDataTypes: [.barcode(symbologies: [.qr])], qualityLevel: .balanced,
                recognizesMultipleItems: false, isHighFrameRateTrackingEnabled: false, isHighlightingEnabled: true)
            scanner.delegate = context.coordinator
            try? scanner.startScanning()
            return scanner
        }

        func updateUIViewController(_ scanner: DataScannerViewController, context: Context) {}

        static func dismantleUIViewController(_ scanner: DataScannerViewController, coordinator: Coordinator) {
            scanner.stopScanning()
        }

        func makeCoordinator() -> Coordinator { Coordinator(onPayload: onPayload) }

        final class Coordinator: NSObject, DataScannerViewControllerDelegate {
            let onPayload: (String) -> Void
            private var delivered = Set<String>()

            init(onPayload: @escaping (String) -> Void) {
                self.onPayload = onPayload
            }

            func dataScanner(
                _ dataScanner: DataScannerViewController, didAdd addedItems: [RecognizedItem],
                allItems: [RecognizedItem]
            ) {
                for case .barcode(let barcode) in addedItems {
                    guard let payload = barcode.payloadStringValue, delivered.insert(payload).inserted else {
                        continue
                    }
                    onPayload(payload)
                }
            }
        }
    }
#endif
