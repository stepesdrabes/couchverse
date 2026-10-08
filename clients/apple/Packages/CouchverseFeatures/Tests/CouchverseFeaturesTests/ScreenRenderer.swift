import SnapshotTesting
import SwiftUI
import UIKit

/// Renders a screen the way SnapshotTesting's view-controller strategy does (a window of the
/// device's size and safe area, the layer drawn at scale 1), but lets SwiftUI settle first and
/// renders again when it catches a frame before SwiftUI drew anything: package tests have no host
/// app, so `drawHierarchy` is not available and an early layer render comes out blank.
@MainActor
enum ScreenRenderer {
    /// The scale of the simulator's screen, which text is rasterized at.
    static var screenScale: CGFloat {
        UIWindow(frame: .zero).screen.scale
    }

    static func image(of view: some View, config: ViewImageConfig, traits: UITraitCollection) -> UIImage {
        let size = config.size ?? CGSize(width: 390, height: 844)
        let controller = UIHostingController(rootView: view)
        controller.additionalSafeAreaInsets = config.safeArea
        let window = UIWindow(frame: CGRect(origin: .zero, size: size))
        window.traitOverrides.preferredContentSizeCategory = traits.preferredContentSizeCategory
        window.traitOverrides.userInterfaceStyle = traits.userInterfaceStyle
        window.traitOverrides.displayScale = 1
        window.traitOverrides.userInterfaceIdiom = config.traits.userInterfaceIdiom
        window.traitOverrides.horizontalSizeClass = config.traits.horizontalSizeClass
        window.traitOverrides.verticalSizeClass = config.traits.verticalSizeClass
        // tvOS lists ignore the trait override alone
        window.overrideUserInterfaceStyle = traits.userInterfaceStyle
        window.rootViewController = controller
        window.isHidden = false
        defer {
            window.isHidden = true
            window.rootViewController = nil
        }

        let format = UIGraphicsImageRendererFormat(for: UITraitCollection(displayScale: 1))
        var image = UIImage()
        for _ in 0..<6 {
            RunLoop.main.run(until: Date(timeIntervalSinceNow: 0.15))
            controller.view.frame = window.bounds
            controller.view.layoutIfNeeded()
            image = UIGraphicsImageRenderer(size: size, format: format).image { context in
                window.layer.render(in: context.cgContext)
            }
            if !isUniform(image) {
                break
            }
        }
        return image
    }

    /// A frame of one colour: nothing has been drawn yet (every screen has text on it).
    private static func isUniform(_ image: UIImage) -> Bool {
        guard let cgImage = image.cgImage, let data = cgImage.dataProvider?.data,
            let bytes = CFDataGetBytePtr(data)
        else { return true }
        let bytesPerRow = cgImage.bytesPerRow
        let bytesPerPixel = cgImage.bitsPerPixel / 8
        func pixel(_ x: Int, _ y: Int) -> UInt32 {
            let offset = y * bytesPerRow + x * bytesPerPixel
            return (0..<min(bytesPerPixel, 4)).reduce(UInt32(0)) { $0 << 8 | UInt32(bytes[offset + $1]) }
        }
        let first = pixel(0, 0)
        for y in stride(from: 0, to: cgImage.height, by: max(cgImage.height / 40, 1)) {
            for x in stride(from: 0, to: cgImage.width, by: max(cgImage.width / 40, 1)) where pixel(x, y) != first {
                return false
            }
        }
        return true
    }
}
