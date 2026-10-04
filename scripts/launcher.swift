// Calendar TUI.app 실행기. 캘린더(calendar, Go TUI)는 Terminal 창에서 돌고, 이 실행기는 그동안 같이 떠 있어
// Dock에 실행 중 점을 보여 주고 Dock 클릭·Cmd+Tab 때 캘린더 창을 앞으로 가져온다. 캘린더가 끝나면 같이 끝난다.
// 창·터미널 화면은 없다(그리는 건 Terminal). 빌드: scripts/package.sh
import AppKit

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

final class Delegate: NSObject, NSApplicationDelegate {
    var exitWatch: DispatchSourceProcess?
    var pid: pid_t = 0

    func applicationDidFinishLaunching(_ n: Notification) {
        showCalendar()
        watch(tries: 20)
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

    // Dock에서 "종료"하면 캘린더도 끈다(입력 소스 복구 후 종료)
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
