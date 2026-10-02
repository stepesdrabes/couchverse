// swift-tools-version: 6.2
import PackageDescription

// `CouchverseCoreFFI.xcframework` and `Sources/CouchverseCore/FFI/` come from
// `make core-apple`; the message types in `Generated/` from `make contract`.
let package = Package(
    name: "CouchverseCore",
    platforms: [.iOS("27.0"), .tvOS("27.0"), .macOS("26.0")],
    products: [
        .library(name: "CouchverseCore", targets: ["CouchverseCore"])
    ],
    targets: [
        .binaryTarget(name: "CouchverseCoreFFI", path: "CouchverseCoreFFI.xcframework"),
        .target(
            name: "CouchverseCore",
            dependencies: ["CouchverseCoreFFI"],
            swiftSettings: [.unsafeFlags(["-warnings-as-errors"])]
        ),
        .testTarget(
            name: "CouchverseCoreTests",
            dependencies: ["CouchverseCore"],
            swiftSettings: [.unsafeFlags(["-warnings-as-errors"])]
        ),
    ],
    swiftLanguageModes: [.v6]
)
