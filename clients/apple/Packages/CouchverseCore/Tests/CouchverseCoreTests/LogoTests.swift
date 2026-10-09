import Foundation
import SwiftUI
import Testing

@testable import CouchverseShared

/// The logo the apps and their widgets draw, held against the web's `logo.svg`.
struct LogoTests {
    /// `clients/web/static/logo.svg`, found from this file.
    static func webLogo() throws -> String {
        var clients = URL(filePath: #filePath)
        for _ in 0..<6 {
            clients.deleteLastPathComponent()
        }
        return try String(contentsOf: clients.appending(path: "web/static/logo.svg"), encoding: .utf8)
    }

    static func attribute(_ name: String, in svg: String) throws -> String {
        let start = try #require(svg.range(of: " \(name)=\""))
        let end = try #require(svg[start.upperBound...].firstIndex(of: "\""))
        return String(svg[start.upperBound..<end])
    }

    @Test func itIsTheWebsLogo() throws {
        let svg = try Self.webLogo()
        #expect(try Self.attribute("viewBox", in: svg) == "0 0 1046 745")
        #expect(
            CouchverseLogo.pathData.replacingOccurrences(of: "\n", with: " ") == (try Self.attribute("d", in: svg)),
            "paste the new logo's d attribute into CouchverseLogoPath.swift")
    }

    @Test func everyCommandInItIsDrawn() {
        let commands = Set(CouchverseLogo.commands(CouchverseLogo.pathData).map(\.command))
        #expect(commands.isSubset(of: ["M", "L", "H", "V", "C", "Z"]))
    }

    @Test func theOutlineSpansTheViewBox() {
        let bounds = CouchverseLogo.outline.boundingRect
        #expect(abs(bounds.minX) < 1 && abs(bounds.minY) < 1)
        #expect(abs(bounds.maxX - 1046) < 2 && abs(bounds.maxY - 745) < 2)
    }

    @Test func itFitsItsFrameCentred() {
        let square = CouchverseLogo().path(in: CGRect(x: 10, y: 10, width: 200, height: 200)).boundingRect
        #expect(abs(square.width - 200) < 1)
        #expect(abs(square.height - 200 / CouchverseLogo.aspectRatio) < 1)
        #expect(abs(square.midX - 110) < 1 && abs(square.midY - 110) < 1)
        let wide = CouchverseLogo().path(in: CGRect(x: 0, y: 0, width: 400, height: 100)).boundingRect
        #expect(abs(wide.height - 100) < 1 && abs(wide.midX - 200) < 1)
    }

    @Test func numbersSplitAsInSvg() {
        let commands = CouchverseLogo.commands("M1.5.5-2e-1 7L3,4Z")
        #expect(commands.map(\.command) == ["M", "L", "Z"])
        #expect(commands.map(\.numbers) == [[1.5, 0.5, -0.2, 7], [3, 4], []])
    }

    @Test func aMoveCarriesLinesAndCloses() {
        let path = CouchverseLogo.parse("M0 0 10 0V10H0Z")
        #expect(path.boundingRect == CGRect(x: 0, y: 0, width: 10, height: 10))
        #expect(path.contains(CGPoint(x: 5, y: 5)) && !path.contains(CGPoint(x: 15, y: 5)))
    }
}
