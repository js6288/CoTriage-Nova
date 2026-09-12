import Foundation

enum Config {
    /// Host backend base URL — auto-configured by start.sh
    /// Simulator: localhost. Physical device: LAN IP.
    static let backendModeURL = "http://10.253.96.16:8765"
}
