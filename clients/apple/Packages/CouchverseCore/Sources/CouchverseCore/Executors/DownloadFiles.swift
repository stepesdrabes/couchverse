import Foundation

/// The downloads directory: finished files under the names the core gives them, kept out of
/// backups (they come from the server again), and beside each interrupted transfer the resume
/// data the system left to continue it from.
public struct DownloadFiles: Sendable {
    public let directory: URL

    public init(directory: URL) {
        self.directory = directory
    }

    /// Where the file `name` lives; `nil` for a name that is not a plain file name.
    public func url(for name: String) -> URL? {
        guard !name.isEmpty, !name.hasPrefix("."), !name.contains("/") else { return nil }
        return directory.appending(path: name, directoryHint: .notDirectory)
    }

    /// The finished file's size, or `nil` when there is none.
    public func size(of name: String) -> UInt64? {
        let values = try? url(for: name)?.resourceValues(forKeys: [.isRegularFileKey, .fileSizeKey])
        guard values?.isRegularFile == true else { return nil }
        return values?.fileSize.map { UInt64(max($0, 0)) }
    }

    /// Deletes a finished file and anything an interrupted transfer of it left.
    public func remove(_ name: String) {
        if let url = url(for: name) {
            try? FileManager.default.removeItem(at: url)
        }
        removeResumeData(for: name)
    }

    /// Moves a fetched file into place as `name`, replacing an earlier copy, and returns its size.
    func keep(_ location: URL, as name: String) throws -> UInt64 {
        guard var target = url(for: name) else { throw CocoaError(.fileWriteInvalidFileName) }
        try prepareDirectory()
        try? FileManager.default.removeItem(at: target)
        try FileManager.default.moveItem(at: location, to: target)
        var values = URLResourceValues()
        values.isExcludedFromBackup = true
        try target.setResourceValues(values)
        removeResumeData(for: name)
        return size(of: name) ?? 0
    }

    /// What a finished transfer amounts to: the file, kept as `name`, when the server sent it;
    /// any other answer is the server refusing (an expired grant, a file it no longer has).
    func finished(_ location: URL, as name: String, status: Int?) -> TransferEvent.Kind {
        guard let status, (200..<300).contains(status) else { return .refused(status: status ?? 0) }
        do {
            return .finished(bytes: try keep(location, as: name))
        } catch {
            return .failed(message: String(describing: error), noSpace: Self.isNoSpace(error), resumable: false)
        }
    }

    /// What an interrupted transfer amounts to; the resume data the system left is kept, so the
    /// next start of `name` continues where this one stopped, even after a relaunch.
    func interrupted(_ name: String, by error: any Error) -> TransferEvent.Kind {
        let resumeData = (error as? URLError)?.downloadTaskResumeData
        if let resumeData {
            saveResumeData(resumeData, for: name)
        }
        return .failed(
            message: error.localizedDescription, noSpace: Self.isNoSpace(error), resumable: resumeData != nil)
    }

    func resumeData(for name: String) -> Data? {
        resumeURL(for: name).flatMap { try? Data(contentsOf: $0) }
    }

    func saveResumeData(_ data: Data, for name: String) {
        guard let url = resumeURL(for: name) else { return }
        try? prepareDirectory()
        try? data.write(to: url, options: .atomic)
    }

    func removeResumeData(for name: String) {
        if let url = resumeURL(for: name) {
            try? FileManager.default.removeItem(at: url)
        }
    }

    private func resumeURL(for name: String) -> URL? {
        url(for: name).map { $0.appendingPathExtension("resume") }
    }

    private func prepareDirectory() throws {
        var directory = directory
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        var values = URLResourceValues()
        values.isExcludedFromBackup = true
        try directory.setResourceValues(values)
    }

    /// Whether `error` means the device is out of space, which trying again will not fix.
    static func isNoSpace(_ error: any Error) -> Bool {
        var current: NSError? = error as NSError
        while let error = current {
            switch (error.domain, error.code) {
            case (NSCocoaErrorDomain, CocoaError.fileWriteOutOfSpace.rawValue), (NSPOSIXErrorDomain, Int(ENOSPC)),
                // a download task only fails to write its own file when the disk is full
                (NSURLErrorDomain, URLError.cannotWriteToFile.rawValue):
                return true
            default:
                current = error.userInfo[NSUnderlyingErrorKey] as? NSError
            }
        }
        return false
    }
}
