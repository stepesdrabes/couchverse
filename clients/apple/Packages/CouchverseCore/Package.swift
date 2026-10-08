// swift-tools-version: 6.2
import PackageDescription

// `CouchverseCoreFFI.xcframework` and `Sources/CouchverseCore/FFI/` come from
// `make core-apple`; the message types in `Generated/` from `make contract`. `CouchverseShared` is
// what the apps share with their extensions (the widget, Top Shelf), which never run the core.
let package = Package(
    name: "CouchverseCore",
    platforms: [.iOS("27.0"), .tvOS("27.0"), .macOS("26.0")],
    products: [
        .library(name: "CouchverseCore", targets: ["CouchverseCore"]),
        .library(name: "CouchverseShared", targets: ["CouchverseShared"]),
    ],
    targets: [
        .binaryTarget(name: "CouchverseCoreFFI", path: "CouchverseCoreFFI.xcframework"),
        .target(
            name: "CouchverseShared",
            swiftSettings: [.unsafeFlags(["-warnings-as-errors"])]
        ),
        .target(
            name: "CouchverseCore",
            dependencies: ["CouchverseCoreFFI", "CouchverseShared"],
            swiftSettings: [.unsafeFlags(["-warnings-as-errors"])]
        ),
        .testTarget(
            name: "CouchverseCoreTests",
            dependencies: ["CouchverseCore", "CouchverseShared"],
            swiftSettings: [.unsafeFlags(["-warnings-as-errors"])]
        ),
    ],
    swiftLanguageModes: [.v6]
)
