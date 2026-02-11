import Foundation
import CallKit

/// Call Directory extension that provides caller ID identification for
/// incoming calls from PBX contacts not in the user's address book.
///
/// The main FlowPBX app writes contact entries to the shared App Group
/// UserDefaults, then triggers a reload. This extension reads those entries
/// and feeds them to iOS for caller identification.
final class CallDirectoryHandler: CXCallDirectoryProvider {
    private static let appGroupId = "group.com.flowpbx.mobile"
    private static let entriesKey = "callerIdEntries"

    private var sharedDefaults: UserDefaults? {
        return UserDefaults(suiteName: Self.appGroupId)
    }

    override func beginRequest(with context: CXCallDirectoryExtensionContext) {
        context.delegate = self

        if context.isIncremental {
            addAllIdentificationPhoneNumbers(to: context)
        } else {
            addAllIdentificationPhoneNumbers(to: context)
        }

        context.completeRequest()
    }

    // MARK: - Identification entries

    /// Read all caller ID entries from shared storage and add them to the
    /// extension context. Entries must be sorted in ascending phone number order.
    private func addAllIdentificationPhoneNumbers(
        to context: CXCallDirectoryExtensionContext
    ) {
        guard let entries = sharedDefaults?.array(forKey: Self.entriesKey)
                as? [[String: Any]] else {
            return
        }

        for entry in entries {
            autoreleasepool {
                guard let number = entry["number"] as? Int64,
                      let label = entry["label"] as? String else { return }
                context.addIdentificationEntry(
                    withNextSequentialPhoneNumber: CXCallDirectoryPhoneNumber(number),
                    label: label
                )
            }
        }
    }
}

// MARK: - CXCallDirectoryExtensionContextDelegate

extension CallDirectoryHandler: CXCallDirectoryExtensionContextDelegate {
    func requestFailed(
        for extensionContext: CXCallDirectoryExtensionContext,
        withError error: Error
    ) {
        NSLog("CallDirectoryHandler request failed: \(error.localizedDescription)")
    }
}
