import Cocoa

// Persistent background app — receives lfy:// URLs and forwards to `lfy open`.
// Stays resident after handling so subsequent clicks are instant.
class AppDelegate: NSObject, NSApplicationDelegate {
    private lazy var lfyPath: String? = findLinkify()

    func application(_ application: NSApplication, open urls: [URL]) {
        guard let url = urls.first, let lfy = lfyPath else { return }

        // Dispatch to background so we don't block the run loop
        DispatchQueue.global().async {
            let task = Process()
            task.executableURL = URL(fileURLWithPath: lfy)
            task.arguments = ["open", url.absoluteString]
            try? task.run()
            task.waitUntilExit()
        }
    }
}

func findLinkify() -> String? {
    let home = FileManager.default.homeDirectoryForCurrentUser.path
    let candidates = [
        "\(home)/.local/bin/lfy",
        "/opt/homebrew/bin/lfy",
        "/usr/local/bin/lfy",
    ]
    for path in candidates {
        if FileManager.default.isExecutableFile(atPath: path) {
            return path
        }
    }
    return nil
}

let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.run()
