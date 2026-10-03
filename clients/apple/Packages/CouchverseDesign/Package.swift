// swift-tools-version: 6.2
import PackageDescription

// Tokens, typography, the accent environment and the shared components. iOS and tvOS only: build
// and test it with `xcodebuild -scheme CouchverseDesign` (see docs/apple.md).
let package = Package(
    name: "CouchverseDesign",
    defaultLocalization: "en",
    platforms: [.iOS("27.0"), .tvOS("27.0")],
    products: [
        .library(name: "CouchverseDesign", targets: ["CouchverseDesign"])
    ],
    dependencies: [
        .package(path: "../CouchverseCore")
    ],
    targets: [
        .target(
            name: "CouchverseDesign",
            dependencies: [.product(name: "CouchverseCore", package: "CouchverseCore")],
            resources: [.process("Generated/Localizable.xcstrings")],
            swiftSettings: [.defaultIsolation(MainActor.self), .treatAllWarnings(as: .error)]
        ),
        .testTarget(
            name: "CouchverseDesignTests",
            dependencies: ["CouchverseDesign"],
            swiftSettings: [.defaultIsolation(MainActor.self), .treatAllWarnings(as: .error)]
        ),
    ],
    swiftLanguageModes: [.v6]
)
