import CallKit
import Flutter

/// Manages CallKit integration for native iOS call UI.
///
/// Provides incoming call display on lock screen, system call UI integration,
/// and handles user actions (answer, end, mute, hold, DTMF) from the native UI.
///
/// Supports push-woken incoming calls: when a VoIP push arrives while the app
/// is suspended, CallKit must be notified immediately (same run loop). User
/// actions (answer/end) that arrive before the Flutter engine is ready are
/// queued and delivered once the method channel is set.
class CallKitManager: NSObject {
    private let provider: CXProvider
    private let callController = CXCallController()
    private var channel: FlutterMethodChannel?
    private var activeCallUUID: UUID?

    /// Actions queued before the Flutter method channel was available.
    private var pendingActions: [(String, Any?)] = []

    override init() {
        let config = CXProviderConfiguration()
        config.localizedName = "FlowPBX"
        config.supportsVideo = false
        config.maximumCallsPerCallGroup = 1
        config.maximumCallGroups = 1
        config.supportedHandleTypes = [.phoneNumber, .generic]
        config.includesCallsInRecents = true

        provider = CXProvider(configuration: config)
        super.init()
        provider.setDelegate(self, queue: nil)
    }

    /// Set the Flutter method channel for communicating actions back to Dart.
    /// Flushes any queued actions that arrived before the channel was ready.
    func setChannel(_ channel: FlutterMethodChannel) {
        self.channel = channel
        flushPendingActions()
    }

    // MARK: - Dart → Native calls

    /// Report an incoming call to CallKit (shows native incoming call UI).
    /// This can be called from PushKit before the Flutter engine is ready.
    func reportIncomingCall(uuid: UUID, handle: String, displayName: String?,
                            completion: @escaping (Error?) -> Void) {
        let update = CXCallUpdate()
        update.remoteHandle = CXHandle(type: .generic, value: handle)
        update.localizedCallerName = displayName
        update.hasVideo = false
        update.supportsGrouping = false
        update.supportsUngrouping = false
        update.supportsDTMF = true
        update.supportsHolding = true

        activeCallUUID = uuid

        provider.reportNewIncomingCall(with: uuid, update: update) { error in
            if error != nil {
                self.activeCallUUID = nil
            }
            completion(error)
        }
    }

    /// Report an outgoing call started via CXCallController.
    ///
    /// This requests CallKit to begin the outgoing call flow:
    /// 1. CXStartCallAction is sent to the provider delegate
    /// 2. The delegate configures audio and fulfills the action
    /// 3. We send a CXCallUpdate so the system UI shows the callee info
    func reportOutgoingCall(uuid: UUID, handle: String) {
        activeCallUUID = uuid
        let cxHandle = CXHandle(type: .generic, value: handle)
        let action = CXStartCallAction(call: uuid, handle: cxHandle)
        action.isVideo = false
        let transaction = CXTransaction(action: action)
        callController.request(transaction) { error in
            if let error = error {
                self.activeCallUUID = nil
                self.sendToFlutter("onCallKitError", arguments: error.localizedDescription)
                return
            }
            // Update the call info so the system UI shows the callee handle
            // instead of "Unknown".
            let update = CXCallUpdate()
            update.remoteHandle = cxHandle
            update.localizedCallerName = handle
            update.hasVideo = false
            update.supportsDTMF = true
            update.supportsHolding = true
            update.supportsGrouping = false
            update.supportsUngrouping = false
            self.provider.reportCall(with: uuid, updated: update)
        }
    }

    /// Notify CallKit that the outgoing call is ringing at the remote end.
    func reportOutgoingCallStartedConnecting(uuid: UUID) {
        provider.reportOutgoingCall(with: uuid, startedConnectingAt: Date())
    }

    /// Notify CallKit that the outgoing call has connected.
    func reportOutgoingCallConnected(uuid: UUID) {
        provider.reportOutgoingCall(with: uuid, connectedAt: Date())
    }

    /// Notify CallKit that the call has ended.
    func reportCallEnded(uuid: UUID, reason: CXCallEndedReason) {
        provider.reportCall(with: uuid, endedAt: Date(), reason: reason)
        activeCallUUID = nil
    }

    /// Request CallKit to end the call (user-initiated from Dart).
    func endCall(uuid: UUID) {
        let action = CXEndCallAction(call: uuid)
        let transaction = CXTransaction(action: action)
        callController.request(transaction) { error in
            if let error = error {
                self.sendToFlutter("onCallKitError", arguments: error.localizedDescription)
            }
        }
    }

    /// Request CallKit to set the mute state.
    func setMuted(uuid: UUID, muted: Bool) {
        let action = CXSetMutedCallAction(call: uuid, muted: muted)
        let transaction = CXTransaction(action: action)
        callController.request(transaction) { _ in }
    }

    /// Request CallKit to set the hold state.
    func setHeld(uuid: UUID, held: Bool) {
        let action = CXSetHeldCallAction(call: uuid, onHold: held)
        let transaction = CXTransaction(action: action)
        callController.request(transaction) { _ in }
    }

    var currentCallUUID: UUID? {
        return activeCallUUID
    }

    // MARK: - Channel message delivery

    /// Send a method call to Flutter, or queue it if the channel is not yet ready.
    private func sendToFlutter(_ method: String, arguments: Any? = nil) {
        if let channel = channel {
            channel.invokeMethod(method, arguments: arguments)
        } else {
            pendingActions.append((method, arguments))
        }
    }

    /// Flush queued actions to the Flutter channel.
    private func flushPendingActions() {
        guard let channel = channel else { return }
        for (method, arguments) in pendingActions {
            channel.invokeMethod(method, arguments: arguments)
        }
        pendingActions.removeAll()
    }
}

// MARK: - CXProviderDelegate

extension CallKitManager: CXProviderDelegate {
    func providerDidReset(_ provider: CXProvider) {
        activeCallUUID = nil
        sendToFlutter("onCallKitReset")
    }

    func provider(_ provider: CXProvider, perform action: CXAnswerCallAction) {
        sendToFlutter("onCallKitAnswer", arguments: action.callUUID.uuidString)
        action.fulfill()
    }

    func provider(_ provider: CXProvider, perform action: CXEndCallAction) {
        sendToFlutter("onCallKitEnd", arguments: action.callUUID.uuidString)
        activeCallUUID = nil
        action.fulfill()
    }

    func provider(_ provider: CXProvider, perform action: CXSetMutedCallAction) {
        sendToFlutter("onCallKitMute", arguments: [
            "uuid": action.callUUID.uuidString,
            "muted": action.isMuted,
        ])
        action.fulfill()
    }

    func provider(_ provider: CXProvider, perform action: CXSetHeldCallAction) {
        sendToFlutter("onCallKitHold", arguments: [
            "uuid": action.callUUID.uuidString,
            "held": action.isOnHold,
        ])
        action.fulfill()
    }

    func provider(_ provider: CXProvider, perform action: CXPlayDTMFCallAction) {
        sendToFlutter("onCallKitDTMF", arguments: [
            "uuid": action.callUUID.uuidString,
            "digits": action.digits,
        ])
        action.fulfill()
    }

    func provider(_ provider: CXProvider, perform action: CXStartCallAction) {
        // Configure audio session for VoIP before fulfilling.
        let session = AVAudioSession.sharedInstance()
        do {
            try session.setCategory(
                .playAndRecord,
                mode: .voiceChat,
                options: [.allowBluetooth, .allowBluetoothA2DP]
            )
        } catch {
            // Non-fatal: audio may already be configured.
        }

        activeCallUUID = action.callUUID

        // If this action was initiated by the system (e.g. user tapped a call
        // in iOS Recents), notify Dart so the SIP stack can place the call.
        let handle = action.handle.value
        sendToFlutter("onCallKitStartCall", arguments: [
            "uuid": action.callUUID.uuidString,
            "handle": handle,
        ])

        action.fulfill()
    }

    func provider(_ provider: CXProvider, didActivate audioSession: AVAudioSession) {
        sendToFlutter("onCallKitAudioActivated")
    }

    func provider(_ provider: CXProvider, didDeactivate audioSession: AVAudioSession) {
        sendToFlutter("onCallKitAudioDeactivated")
    }
}
