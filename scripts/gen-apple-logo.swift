// Makes the Apple apps' logo assets from clients/web/static/logo.svg: the path CouchverseLogo fills,
// the app icons, the tvOS image stacks and the Top Shelf images.
//
// usage (from the repository root): swift scripts/gen-apple-logo.swift
// No SVG rasterizer is needed: the logo is one path of absolute M, L, H, V, C and Z commands,
// drawn with Core Graphics in the default accent and its on-accent white.
import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

let accent = color(0xE5, 0x09, 0x14)
let onAccent = color(0xFF, 0xFF, 0xFF)
/// `color.bg`, the dark end of the TV gradients (Android's TV banner fades the same way).
let night = color(0x07, 0x08, 0x0D)

let apple = "clients/apple"
let brand = "\(apple)/CouchverseTV/Assets.xcassets/App Icon & Top Shelf Image.brandassets"

let svg = try String(contentsOfFile: "clients/web/static/logo.svg", encoding: .utf8)
let viewBox = attribute("viewBox", in: svg).split(separator: " ").compactMap { Double($0) }
guard viewBox.count == 4 else { fail("logo.svg has no viewBox") }
let logoSize = CGSize(width: viewBox[2], height: viewBox[3])
let pathData = attribute("d", in: svg)
let logo = outline(pathData)

// the shape the apps and their widgets draw, the same path wrapped at its spaces
let swift = "\(apple)/Packages/CouchverseCore/Sources/CouchverseShared/CouchverseLogoPath.swift"
let lines = wrapped(pathData, width: 110).map { "        \($0)" }.joined(separator: "\n")
let source = """
    extension CouchverseLogo {
        /// The `d` attribute of `clients/web/static/logo.svg`, written by `scripts/gen-apple-logo.swift`
        /// (`LogoTests` checks that the two still match).
        static let pathData = \"\"\"
    \(lines)
            \"\"\"
    }

    """
try source.write(toFile: swift, atomically: true, encoding: .utf8)
print("wrote \(swift)")

// iPhone and iPad: the white logo across two thirds of the accent square, like Android's launcher
// icon; dark and tinted on a clear background, which the system fills
let icons = "\(apple)/Couchverse/Assets.xcassets/AppIcon.appiconset"
write(icon(fill: accent, logo: onAccent), "\(icons)/AppIcon.png")
write(icon(fill: nil, logo: accent), "\(icons)/AppIcon-Dark.png")
write(icon(fill: nil, logo: onAccent), "\(icons)/AppIcon-Tinted.png")

// Apple TV: the gradient behind, the logo in front so the focused icon has parallax
for (stack, size, scales) in [
    ("App Icon", CGSize(width: 400, height: 240), [1, 2]),
    ("App Icon - App Store", CGSize(width: 1280, height: 768), [1]),
] {
    for scale in scales {
        let pixels = size * CGFloat(scale)
        func layer(_ name: String, _ type: String) -> String {
            "\(brand)/\(stack).imagestack/\(name).imagestacklayer/Content.imageset/\(name)@\(scale)x.\(type)"
        }
        write(render(pixels, opaque: true) { tvBackdrop($0, pixels) }, layer("Back", "jpg"))
        write(
            render(pixels, opaque: false) { drawLogo($0, height: pixels.height * 0.48, in: pixels, color: onAccent) },
            layer("Front", "png"))
    }
}

// the Top Shelf before the app has anything to show there
for (imageset, size) in [
    ("Top Shelf Image", CGSize(width: 1920, height: 720)),
    ("Top Shelf Image Wide", CGSize(width: 2320, height: 720)),
] {
    for scale in [1, 2] {
        let pixels = size * CGFloat(scale)
        let image = render(pixels, opaque: true) { context in
            tvBackdrop(context, pixels)
            drawLogo(context, height: pixels.height * 0.4, in: pixels, color: onAccent)
        }
        write(image, "\(brand)/\(imageset).imageset/TopShelf@\(scale)x.jpg")
    }
}

/// A 1024-point iOS icon: `fill` behind (none leaves it clear), the logo two thirds of its width.
func icon(fill: CGColor?, logo color: CGColor) -> CGImage {
    let size = CGSize(width: 1024, height: 1024)
    return render(size, opaque: fill != nil) { context in
        if let fill {
            context.setFillColor(fill)
            context.fill(CGRect(origin: .zero, size: size))
        }
        drawLogo(context, height: size.width * 2 / 3 * logoSize.height / logoSize.width, in: size, color: color)
    }
}

/// The accent from the top-leading corner into the night at the bottom-trailing one.
func tvBackdrop(_ context: CGContext, _ size: CGSize) {
    let colors = [accent, accent, night] as CFArray
    let gradient = CGGradient(colorsSpace: sRGB, colors: colors, locations: [0, 0.2, 1])!
    context.drawLinearGradient(
        gradient, start: .zero, end: CGPoint(x: size.width, y: size.height),
        options: [.drawsBeforeStartLocation, .drawsAfterEndLocation])
}

/// The logo `height` tall, centred in `size`.
func drawLogo(_ context: CGContext, height: CGFloat, in size: CGSize, color: CGColor) {
    let scale = height / logoSize.height
    context.saveGState()
    context.translateBy(
        x: (size.width - logoSize.width * scale) / 2, y: (size.height - logoSize.height * scale) / 2)
    context.scaleBy(x: scale, y: scale)
    context.addPath(logo)
    context.setFillColor(color)
    context.fillPath()
    context.restoreGState()
}

/// An sRGB bitmap drawn with SVG's top-left origin; an opaque one has no alpha channel, as the App
/// Store requires of the iOS icon.
func render(_ size: CGSize, opaque: Bool, draw: (CGContext) -> Void) -> CGImage {
    let alpha = opaque ? CGImageAlphaInfo.noneSkipLast : .premultipliedLast
    guard
        let context = CGContext(
            data: nil, width: Int(size.width), height: Int(size.height), bitsPerComponent: 8, bytesPerRow: 0,
            space: sRGB, bitmapInfo: alpha.rawValue)
    else { fail("cannot draw \(size)") }
    context.translateBy(x: 0, y: size.height)
    context.scaleBy(x: 1, y: -1)
    draw(context)
    return context.makeImage()!
}

/// PNG, or JPEG for a `.jpg` path.
func write(_ image: CGImage, _ path: String) {
    let jpeg = path.hasSuffix(".jpg")
    let url = URL(fileURLWithPath: path)
    guard
        let destination = CGImageDestinationCreateWithURL(
            url as CFURL, (jpeg ? UTType.jpeg : UTType.png).identifier as CFString, 1, nil)
    else { fail("cannot write \(path)") }
    let options = jpeg ? [kCGImageDestinationLossyCompressionQuality: 0.92] as CFDictionary : nil
    CGImageDestinationAddImage(destination, image, options)
    guard CGImageDestinationFinalize(destination) else { fail("cannot write \(path)") }
    print("wrote \(path) (\(image.width)x\(image.height))")
}

/// `text` broken at its spaces into lines of at most `width` characters.
func wrapped(_ text: String, width: Int) -> [String] {
    var lines: [String] = []
    var line = ""
    for word in text.split(separator: " ", omittingEmptySubsequences: false) {
        if !line.isEmpty && line.count + 1 + word.count > width {
            lines.append(line)
            line = String(word)
        } else {
            line = line.isEmpty ? String(word) : "\(line) \(word)"
        }
    }
    return lines + [line]
}

/// The value of the first `name="..."` in `text`.
func attribute(_ name: String, in text: String) -> String {
    guard let start = text.range(of: " \(name)=\""), let end = text[start.upperBound...].firstIndex(of: "\"") else {
        fail("logo.svg has no \(name)")
    }
    return String(text[start.upperBound..<end])
}

/// The path in an SVG `d` attribute. Only the commands the logo uses are understood; anything else
/// stops the script rather than drawing the logo wrong.
func outline(_ data: String) -> CGPath {
    var tokens: [String] = []
    var number = ""
    func flush() {
        if !number.isEmpty { tokens.append(number) }
        number = ""
    }
    for character in data {
        switch character {
        case "0"..."9", "e", "E":
            number.append(character)
        case ".":
            // "1.5.5" is two numbers
            if number.contains(".") { flush() }
            number.append(character)
        case "-", "+":
            if let last = number.last, last != "e", last != "E" { flush() }
            number.append(character)
        case " ", ",", "\n", "\r", "\t":
            flush()
        default:
            flush()
            tokens.append(String(character))
        }
    }
    flush()

    let path = CGMutablePath()
    var index = 0
    var current = CGPoint.zero
    var start = CGPoint.zero
    func hasNumber() -> Bool { index < tokens.count && Double(tokens[index]) != nil }
    func next() -> CGFloat {
        guard hasNumber() else { fail("logo.svg: expected a number at token \(index)") }
        index += 1
        return CGFloat(Double(tokens[index - 1])!)
    }
    func point() -> CGPoint { CGPoint(x: next(), y: next()) }
    while index < tokens.count {
        var command = tokens[index]
        index += 1
        repeat {
            switch command {
            case "M":
                current = point()
                start = current
                path.move(to: current)
                // further pairs after a move are lines
                command = "L"
            case "L":
                current = point()
                path.addLine(to: current)
            case "H":
                current.x = next()
                path.addLine(to: current)
            case "V":
                current.y = next()
                path.addLine(to: current)
            case "C":
                let control1 = point()
                let control2 = point()
                current = point()
                path.addCurve(to: current, control1: control1, control2: control2)
            case "Z", "z":
                path.closeSubpath()
                current = start
            default:
                fail("logo.svg uses the path command \(command), which this script does not draw")
            }
        } while hasNumber()
    }
    return path
}

func color(_ red: Int, _ green: Int, _ blue: Int) -> CGColor {
    CGColor(colorSpace: sRGB, components: [CGFloat(red) / 255, CGFloat(green) / 255, CGFloat(blue) / 255, 1])!
}

var sRGB: CGColorSpace { CGColorSpace(name: CGColorSpace.sRGB)! }

func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data("gen-apple-icons: \(message)\n".utf8))
    exit(1)
}

extension CGSize {
    static func * (size: CGSize, factor: CGFloat) -> CGSize {
        CGSize(width: size.width * factor, height: size.height * factor)
    }
}
