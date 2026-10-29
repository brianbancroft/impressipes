// swift-tools-version: 6.0
import PackageDescription

// Generated sources are committed (see ../project.json `codegen`); the generator
// dependency is here so `swift run swift-openapi-generator` works from this package.
let package = Package(
    name: "ApiClient",
    platforms: [.iOS(.v18), .watchOS(.v11), .macOS(.v15)],
    products: [.library(name: "ApiClient", targets: ["ApiClient"])],
    dependencies: [
        .package(url: "https://github.com/apple/swift-openapi-generator", from: "1.14.3"),
        .package(url: "https://github.com/apple/swift-openapi-runtime", from: "1.13.3"),
        .package(url: "https://github.com/apple/swift-openapi-urlsession", from: "1.4.0"),
    ],
    targets: [
        .target(
            name: "ApiClient",
            dependencies: [
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "OpenAPIURLSession", package: "swift-openapi-urlsession"),
            ]
        )
    ]
)
