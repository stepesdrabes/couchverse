import Foundation
import ImageIO
import UniformTypeIdentifiers

/// Picked files on their way to the core's `Upload` effects. The core never holds file bytes: a
/// screen keeps the picked file here and hands the core its file URL as the handle
/// (`ImageChosen`), which the upload effect brings back to `HTTPExecutor`.
public struct UploadFiles: Sendable {
    let directory: URL

    public init(directory: URL = FileManager.default.temporaryDirectory.appending(path: "uploads")) {
        self.directory = directory
    }

    /// A picked photo as a JPEG at most `maxPixels` on its longer side, in place of the one picked
    /// before: servers take JPEG, PNG and WebP while phones shoot HEIC, and a full-size photo is
    /// megabytes the server would only scale down. Returns its handle.
    @concurrent
    public func holdImage(_ data: Data, maxPixels: Int) async throws -> String {
        let jpeg = try Self.jpeg(data, maxPixels: maxPixels)
        let files = FileManager.default
        try? files.removeItem(at: directory)
        try files.createDirectory(at: directory, withIntermediateDirectories: true)
        let file = directory.appending(path: "\(UUID().uuidString).jpg")
        try jpeg.write(to: file, options: .atomic)
        return file.absoluteString
    }

    /// The file behind a handle; its name goes with it, since servers type images by the name.
    static func open(_ handle: String) -> PickedFile? {
        guard let url = URL(string: handle), url.isFileURL, let data = try? Data(contentsOf: url) else {
            return nil
        }
        return PickedFile(name: url.lastPathComponent, data: data)
    }

    static func jpeg(_ data: Data, maxPixels: Int) throws -> Data {
        let options: [CFString: Any] = [
            kCGImageSourceCreateThumbnailFromImageAlways: true,
            // upright, however the camera was held
            kCGImageSourceCreateThumbnailWithTransform: true,
            kCGImageSourceThumbnailMaxPixelSize: maxPixels,
        ]
        guard let source = CGImageSourceCreateWithData(data as CFData, nil),
            let image = CGImageSourceCreateThumbnailAtIndex(source, 0, options as CFDictionary)
        else {
            throw UploadFileError.notAnImage
        }
        let output = NSMutableData()
        guard
            let destination = CGImageDestinationCreateWithData(output, UTType.jpeg.identifier as CFString, 1, nil)
        else {
            throw UploadFileError.notAnImage
        }
        let quality: [CFString: Any] = [kCGImageDestinationLossyCompressionQuality: 0.85]
        CGImageDestinationAddImage(destination, image, quality as CFDictionary)
        guard CGImageDestinationFinalize(destination) else { throw UploadFileError.notAnImage }
        return output as Data
    }
}

public enum UploadFileError: Error {
    case notAnImage
}

/// A picked file, read for its upload.
struct PickedFile: Equatable {
    let name: String
    let data: Data

    var contentType: String {
        UTType(filenameExtension: (name as NSString).pathExtension)?.preferredMIMEType ?? "application/octet-stream"
    }
}

/// A `multipart/form-data` body with one file part.
struct MultipartForm {
    let boundary: String
    let body: Data

    var contentType: String { "multipart/form-data; boundary=\(boundary)" }

    init(field: String, file: PickedFile, boundary: String = "couchverse-\(UUID().uuidString)") {
        self.boundary = boundary
        // a quote or a line break in the name would end the header early
        let name = file.name.filter { !"\"\r\n".contains($0) }
        var body = Data()
        body.append(Data("--\(boundary)\r\n".utf8))
        body.append(Data("Content-Disposition: form-data; name=\"\(field)\"; filename=\"\(name)\"\r\n".utf8))
        body.append(Data("Content-Type: \(file.contentType)\r\n\r\n".utf8))
        body.append(file.data)
        body.append(Data("\r\n--\(boundary)--\r\n".utf8))
        self.body = body
    }
}
