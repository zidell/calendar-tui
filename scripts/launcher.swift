// Calendar TUI.app 실행기. 캘린더(calendar, Go TUI)는 Terminal 창에서 돌고, 이 실행기는 그동안 같이 떠 있어
// Dock에 실행 중 점을 보여 주고 Dock 클릭·Cmd+Tab 때 캘린더 창을 앞으로 가져온다. 캘린더가 끝나면 같이 끝난다.
// 창·터미널 화면은 없다(그리는 건 Terminal). 빌드: scripts/package.sh
import AppKit
import Carbon.HIToolbox

// 바이너리는 번들 밖에 둔다(dev.sh가 갈아 끼워도 앱 서명이 바뀌지 않게). 없으면 번들 안 것을 쓴다.
let supportBin = FileManager.default.homeDirectoryForCurrentUser
    .appendingPathComponent("Library/Application Support/calendar-tui/calendar").path
let bundleBin = Bundle.main.resourceURL!.appendingPathComponent("calendar").path
let bin = FileManager.default.isExecutableFile(atPath: supportBin) ? supportBin : bundleBin

// 마지막 창 크기(글자 칸)와 글꼴 크기. 캘린더가 창 크기는 바뀔 때마다(main.go WindowSizeMsg),
// 글꼴 크기는 종료할 때(main.go termFontSize) state.json에 적는다(config.go). 예전 이름 settings.json도 읽는다.
func savedSize() -> (cols: Int, rows: Int, font: Double)? {
    let dir = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent("Library/Application Support/calendar-tui")
    guard let data = (try? Data(contentsOf: dir.appendingPathComponent("state.json")))
            ?? (try? Data(contentsOf: dir.appendingPathComponent("settings.json"))),
          let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
          let c = obj["windowCols"] as? Int, let r = obj["windowRows"] as? Int, c > 20, r > 10 else { return nil }
    return (c, r, obj["fontSize"] as? Double ?? 0)
}

// Terminal에서 제목 "▦ 캘린더"·"▦ Calendar"(main.go windowTitle) 창을 앞으로, 없으면 새 창에서 실행한다.
// 새 창은 마지막 크기·글꼴 크기로, 기억한 크기가 없으면 화면을 거의 채운다.
func showCalendar() {
    let f = NSScreen.main?.frame.size ?? CGSize(width: 1600, height: 1000)
    var resize = "set bounds of front window to {40, 60, \(Int(f.width) - 40), \(Int(f.height) - 40)}"
    if let s = savedSize() {
        // 글꼴을 먼저 바꿔야 칸 수가 그 글꼴 기준으로 맞는다
        let font = s.font >= 6 && s.font <= 72 ? "set font size of selected tab of front window to \(s.font)\n        " : ""
        resize = """
        \(font)set number of columns of selected tab of front window to \(s.cols)
                set number of rows of selected tab of front window to \(s.rows)
        """
    }
    let cmd = "clear; '\(bin.replacingOccurrences(of: "'", with: "'\\''"))'; exit"
    let src = """
    set wasRunning to application "Terminal" is running
    tell application "Terminal"
        repeat with w in windows
            if name of w contains "▦ 캘린더" or name of w contains "▦ Calendar" then
                set index of w to 1
                activate
                return
            end if
        end repeat
        if wasRunning then
            do script "\(cmd)"
        else
            activate
            delay 0.5
            do script "\(cmd)" in front window
        end if
        \(resize)
        activate
    end tell
    """
    var err: NSDictionary?
    NSAppleScript(source: src)?.executeAndReturnError(&err)
    if let err { NSLog("calendar-tui launcher: %@", err) }
}

// 캘린더 프로세스 pid. 자동 재시작(exec)해도 pid는 그대로다.
func calendarPID() -> pid_t? {
    let p = Process()
    p.executableURL = URL(fileURLWithPath: "/usr/bin/pgrep")
    p.arguments = ["-x", "calendar"]
    let out = Pipe()
    p.standardOutput = out
    try? p.run()
    p.waitUntilExit()
    let s = String(data: out.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
    return s.split(separator: "\n").first.flatMap { pid_t($0) }
}

// Cmd 단축키를 실행기가 가로채 Terminal 창이 단독 앱처럼 굴게 한다(Terminal은 Cmd 조합을 앱에 넘기지 않는다).
//  Cmd+H: Terminal이 맨 앞일 때만. 맨 앞 창이 캘린더면 그 창만 숨기고, 아니면 원래처럼 Terminal을 숨긴다.
//         숨긴 창은 showCalendar(Dock 클릭·Cmd+Tab)가 다시 보인다.
//  Cmd+글자: 캘린더 창이 포커스인 동안만(캘린더가 알려 줌) 받아 캘린더에 그 글자로 넘긴다. 입력기는 Cmd 조합을
//         조합하지 않으므로 한글 상태에서도 단축키가 먹는다.
let hideScript = NSAppleScript(source: """
tell application "Terminal"
    if (count windows) > 0 then
        set w to front window
        if name of w contains "▦ 캘린더" or name of w contains "▦ Calendar" then
            set visible of w to false
            return "calendar"
        end if
    end if
end tell
return "other"
""")

// 단축키는 Carbon RegisterEventHotKey(손쉬운 사용 권한 필요 없음). 눌리면 id로 가른다: 1 = Cmd+H, 100+ = Cmd+글자.
// unixAddr는 유닉스 소켓 주소(경로는 sun_path 길이에서 잘림).
func unixAddr(_ path: String) -> sockaddr_un {
    var addr = sockaddr_un()
    addr.sun_family = sa_family_t(AF_UNIX)
    withUnsafeMutableBytes(of: &addr.sun_path) { dst in
        let src = Array(path.utf8.prefix(dst.count - 1))
        dst.copyBytes(from: src)
    }
    return addr
}

final class HotKeys {
    var hideRef: EventHotKeyRef?
    var keyRefs: [EventHotKeyRef] = []
    var calendarFocused = false
    let supportDir = FileManager.default.homeDirectoryForCurrentUser
        .appendingPathComponent("Library/Application Support/calendar-tui").path
    var sock: Int32 = -1
    var sockSource: DispatchSourceRead?

    // 캘린더 창이 포커스일 때 가져오는 Cmd+글자(물리 키 위치라 한글 자판이어도 같다). 캘린더엔 글자로 넘긴다.
    // Cmd+C·V(복사·붙여넣기), Cmd+H(아래 따로), Cmd+M(최소화)은 Terminal에 그대로 둔다.
    static let keys: [(Int, String)] = [
        (kVK_ANSI_A, "a"), (kVK_ANSI_B, "b"), (kVK_ANSI_D, "d"), (kVK_ANSI_E, "e"), (kVK_ANSI_F, "f"),
        (kVK_ANSI_G, "g"), (kVK_ANSI_I, "i"), (kVK_ANSI_J, "j"), (kVK_ANSI_K, "k"), (kVK_ANSI_L, "l"),
        (kVK_ANSI_N, "n"), (kVK_ANSI_O, "o"), (kVK_ANSI_P, "p"), (kVK_ANSI_Q, "q"), (kVK_ANSI_R, "r"),
        (kVK_ANSI_S, "s"), (kVK_ANSI_T, "t"), (kVK_ANSI_U, "u"), (kVK_ANSI_W, "w"), (kVK_ANSI_X, "x"),
        (kVK_ANSI_Y, "y"), (kVK_ANSI_Z, "z"), (kVK_ANSI_Slash, "/"), (kVK_ANSI_LeftBracket, "["),
        (kVK_ANSI_RightBracket, "]"), (kVK_ANSI_Comma, ","),
        (kVK_ANSI_1, "1"), (kVK_ANSI_2, "2"), (kVK_ANSI_3, "3"),
    ]

    init() {
        var spec = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
        InstallEventHandler(GetApplicationEventTarget(), { _, event, _ in
            var hk = EventHotKeyID()
            GetEventParameter(event, EventParamName(kEventParamDirectObject), EventParamType(typeEventHotKeyID), nil,
                              MemoryLayout<EventHotKeyID>.size, nil, &hk)
            let id = Int(hk.id)
            DispatchQueue.main.async { delegate.hotKeys?.pressed(id) }
            return noErr
        }, 1, &spec, nil, nil)
        let ws = NSWorkspace.shared.notificationCenter
        ws.addObserver(forName: NSWorkspace.didActivateApplicationNotification, object: nil, queue: .main) { [weak self] n in
            let app = n.userInfo?[NSWorkspace.applicationUserInfoKey] as? NSRunningApplication
            self?.terminalFront(app?.bundleIdentifier == "com.apple.Terminal")
        }
        terminalFront(NSWorkspace.shared.frontmostApplication?.bundleIdentifier == "com.apple.Terminal")
        listen()
    }

    // Cmd+H: Terminal이 맨 앞일 때만. 맨 앞 창이 캘린더면 그 창만 숨기고, 아니면 원래처럼 Terminal을 숨긴다.
    func terminalFront(_ on: Bool) {
        if on, hideRef == nil {
            RegisterEventHotKey(UInt32(kVK_ANSI_H), UInt32(cmdKey), EventHotKeyID(signature: OSType(0x6361_6c68), id: 1),
                                GetApplicationEventTarget(), 0, &hideRef)
        } else if !on, let r = hideRef {
            UnregisterEventHotKey(r)
            hideRef = nil
        }
        if !on { setKeys(false) } // 다른 앱으로 가면 Cmd+글자도 확실히 푼다
    }

    // Cmd+글자: 캘린더가 "focus 1"을 보낸 동안만(그 창이 포커스). 다른 Terminal 창·앱은 원래대로.
    func setKeys(_ on: Bool) {
        calendarFocused = on
        if on, keyRefs.isEmpty {
            for (i, k) in HotKeys.keys.enumerated() {
                var ref: EventHotKeyRef?
                RegisterEventHotKey(UInt32(k.0), UInt32(cmdKey), EventHotKeyID(signature: OSType(0x6361_6c68), id: UInt32(100 + i)),
                                    GetApplicationEventTarget(), 0, &ref)
                if let ref { keyRefs.append(ref) }
            }
        } else if !on {
            keyRefs.forEach { UnregisterEventHotKey($0) }
            keyRefs.removeAll()
        }
    }

    func pressed(_ id: Int) {
        if id == 1 {
            var err: NSDictionary?
            if hideScript?.executeAndReturnError(&err).stringValue != "calendar" {
                NSRunningApplication.runningApplications(withBundleIdentifier: "com.apple.Terminal").first?.hide()
            }
            return
        }
        let i = id - 100
        if i >= 0, i < HotKeys.keys.count { send("key " + HotKeys.keys[i].1) }
    }

    // 캘린더 → 실행기: launcher.sock으로 "focus 1"/"focus 0"
    func listen() {
        let path = supportDir + "/launcher.sock"
        unlink(path)
        sock = socket(AF_UNIX, SOCK_DGRAM, 0)
        guard sock >= 0 else { return }
        var addr = unixAddr(path)
        let ok = withUnsafePointer(to: &addr) {
            $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { bind(sock, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
        }
        guard ok == 0 else { return }
        let src = DispatchSource.makeReadSource(fileDescriptor: sock, queue: .main)
        src.setEventHandler { [weak self] in
            guard let self else { return }
            var buf = [UInt8](repeating: 0, count: 64)
            let n = recv(self.sock, &buf, buf.count, 0)
            guard n > 0 else { return }
            let msg = String(decoding: buf[0..<n], as: UTF8.self)
            if msg == "focus 1" { self.setKeys(NSWorkspace.shared.frontmostApplication?.bundleIdentifier == "com.apple.Terminal") }
            if msg == "focus 0" { self.setKeys(false) }
        }
        src.resume()
        sockSource = src
    }

    // 실행기 → 캘린더: calendar.sock으로 "key a"
    func send(_ msg: String) {
        let path = supportDir + "/calendar.sock"
        let s = socket(AF_UNIX, SOCK_DGRAM, 0)
        guard s >= 0 else { return }
        defer { close(s) }
        var addr = unixAddr(path)
        _ = msg.withCString { m in
            withUnsafePointer(to: &addr) {
                $0.withMemoryRebound(to: sockaddr.self, capacity: 1) { sendto(s, m, strlen(m), 0, $0, socklen_t(MemoryLayout<sockaddr_un>.size)) }
            }
        }
    }
}

final class Delegate: NSObject, NSApplicationDelegate {
    var exitWatch: DispatchSourceProcess?
    var pid: pid_t = 0
    var hotKeys: HotKeys?

    func applicationDidFinishLaunching(_ n: Notification) {
        showCalendar()
        watch(tries: 20)
        hotKeys = HotKeys()
    }

    // 캘린더가 뜰 때까지(최대 약 10초) pid를 찾고, 그다음엔 종료 알림만 기다린다(폴링 없음).
    func watch(tries: Int) {
        guard let p = calendarPID() else {
            if tries > 0 {
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) { self.watch(tries: tries - 1) }
            } else {
                NSApp.terminate(nil)
            }
            return
        }
        pid = p
        let src = DispatchSource.makeProcessSource(identifier: p, eventMask: .exit, queue: .main)
        src.setEventHandler { NSApp.terminate(nil) }
        src.resume()
        exitWatch = src
    }

    // Dock 아이콘을 다시 누름
    func applicationShouldHandleReopen(_ app: NSApplication, hasVisibleWindows: Bool) -> Bool {
        showCalendar()
        return false
    }

    // Cmd+Tab 등으로 이 앱이 앞에 옴
    func applicationDidBecomeActive(_ n: Notification) {
        if pid != 0 { showCalendar() }
    }

    // Dock에서 "종료"하면 캘린더도 끈다
    func applicationWillTerminate(_ n: Notification) {
        exitWatch?.cancel()
        if pid != 0 { kill(pid, SIGTERM) }
    }
}

let app = NSApplication.shared
let delegate = Delegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
