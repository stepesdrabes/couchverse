// swift-tools-version: 6.2
import PackageDescription

// The screens, grouped by feature, and the root that composes them. iOS and tvOS only: build and
// test it with `xcodebuild -scheme CouchverseFeatures` (see docs/apple.md). Snapshot references
// live next to the tests in `__Snapshots__`.
let package = Package(
    name: "CouchverseFeatures",
    defaultLocalization: "en",
    platforms: [.iOS("27.0"), .tvOS("27.0")],
    products: [
        .library(name: "CouchverseFeatures", targets: ["CouchverseFeatures"])
    ],
    dependencies: [
        .package(path: "../CouchverseCore"),
        .package(path: "../CouchverseDesign"),
        .package(url: "https://github.com/pointfreeco/swift-snapshot-testing", from: "1.19.6"),
    ],
    targets: [
        .target(
            name: "CouchverseFeatures",
            dependencies: [
                .product(name: "CouchverseCore", package: "CouchverseCore"),
                .product(name: "CouchverseDesign", package: "CouchverseDesign"),
            ],
            swiftSettings: [.defaultIsolation(MainActor.self), .treatAllWarnings(as: .error)]
        ),
        .testTarget(
            name: "CouchverseFeaturesTests",
            dependencies: [
                "CouchverseFeatures",
                .product(name: "SnapshotTesting", package: "swift-snapshot-testing"),
            ],
            exclude: ["__Snapshots__"],
            swiftSettings: [
                .defaultIsolation(MainActor.self), .treatAllWarnings(as: .error),
                // package tests run without a host app, so snapshots need a window without a scene
                .treatWarning("DeprecatedDeclaration", as: .warning),
            ]
        ),
    ],
    swiftLanguageModes: [.v6]
)
