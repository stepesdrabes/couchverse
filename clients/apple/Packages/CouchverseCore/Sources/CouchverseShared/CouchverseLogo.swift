import SwiftUI

/// The Couchverse logo, a couch with a bucket of popcorn on it: the web's `logo.svg` as a shape in
/// one colour, fitted and centred in its frame with its proportions kept. Here rather than in the
/// design package because the widgets draw it too and link nothing else.
public struct CouchverseLogo: Shape {
    /// Width over height.
    public static let aspectRatio = viewBox.width / viewBox.height

    public init() {}

    public func path(in rect: CGRect) -> Path {
        let scale = min(rect.width / Self.viewBox.width, rect.height / Self.viewBox.height)
        let offset = CGPoint(
            x: rect.midX - Self.viewBox.width * scale / 2, y: rect.midY - Self.viewBox.height * scale / 2)
        return Self.outline.applying(
            CGAffineTransform(translationX: offset.x, y: offset.y).scaledBy(x: scale, y: scale))
    }

    static let viewBox = CGSize(width: 1046, height: 745)
    static let outline = parse(pathData)

    /// The path an SVG `d` attribute draws. It reads the absolute moves, lines and cubic curves the
    /// logo is made of and skips any other command, which `LogoTests` would notice.
    static func parse(_ data: String) -> Path {
        var path = Path()
        var current = CGPoint.zero
        var start = CGPoint.zero
        for (command, numbers) in commands(data) {
            switch command {
            case "M", "L":
                for index in stride(from: 0, to: numbers.count - 1, by: 2) {
                    current = CGPoint(x: numbers[index], y: numbers[index + 1])
                    // the pairs after a move's first are lines
                    if command == "M" && index == 0 {
                        path.move(to: current)
                        start = current
                    } else {
                        path.addLine(to: current)
                    }
                }
            case "H":
                for x in numbers {
                    current.x = x
                    path.addLine(to: current)
                }
            case "V":
                for y in numbers {
                    current.y = y
                    path.addLine(to: current)
                }
            case "C":
                for index in stride(from: 0, to: numbers.count - 5, by: 6) {
                    current = CGPoint(x: numbers[index + 4], y: numbers[index + 5])
                    path.addCurve(
                        to: current, control1: CGPoint(x: numbers[index], y: numbers[index + 1]),
                        control2: CGPoint(x: numbers[index + 2], y: numbers[index + 3]))
                }
            case "Z", "z":
                path.closeSubpath()
                current = start
            default:
                break
            }
        }
        return path
    }

    /// A path's commands in order, each with its numbers.
    static func commands(_ data: String) -> [(command: Character, numbers: [CGFloat])] {
        var commands: [(command: Character, numbers: [CGFloat])] = []
        var number = ""
        func flush() {
            if let value = Double(number), !commands.isEmpty {
                commands[commands.count - 1].numbers.append(CGFloat(value))
            }
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
            case _ where character.isLetter:
                flush()
                commands.append((character, []))
            default:
                flush()
            }
        }
        flush()
        return commands
    }
}
