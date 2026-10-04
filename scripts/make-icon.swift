// 앱 아이콘(1024px PNG)을 그린다: 어두운 둥근 사각형 + 빨간 머리띠 + 달력 격자, 오늘 칸은 흰 테두리(앱 화면과 같은 표시).
// 사용: swift scripts/make-icon.swift 출력.png
import AppKit

let size: CGFloat = 1024
let out = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : "icon.png"
let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: Int(size), pixelsHigh: Int(size), bitsPerSample: 8,
                           samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
let ctx = NSGraphicsContext.current!.cgContext

func rgb(_ hex: UInt32, _ a: CGFloat = 1) -> CGColor {
    CGColor(red: CGFloat((hex >> 16) & 0xff) / 255, green: CGFloat((hex >> 8) & 0xff) / 255, blue: CGFloat(hex & 0xff) / 255, alpha: a)
}

// macOS 아이콘 격자: 1024 캔버스에 824 크기 둥근 사각형(위아래 여백 100)
let body = CGRect(x: 100, y: 100, width: 824, height: 824)
let shape = CGPath(roundedRect: body, cornerWidth: 185, cornerHeight: 185, transform: nil)

// 그림자
ctx.saveGState()
ctx.setShadow(offset: CGSize(width: 0, height: -12), blur: 30, color: rgb(0x000000, 0.45))
ctx.addPath(shape); ctx.setFillColor(rgb(0x1e1e1e)); ctx.fillPath()
ctx.restoreGState()

ctx.saveGState()
ctx.addPath(shape); ctx.clip()
// 바탕: 위가 살짝 밝은 어두운 회색
let grad = CGGradient(colorsSpace: CGColorSpaceCreateDeviceRGB(), colors: [rgb(0x2a2a2c), rgb(0x18181a)] as CFArray, locations: [0, 1])!
ctx.drawLinearGradient(grad, start: CGPoint(x: 0, y: body.maxY), end: CGPoint(x: 0, y: body.minY), options: [])
// 빨간 머리띠
let headH: CGFloat = 190
ctx.setFillColor(rgb(0xe5484d)); ctx.fill(CGRect(x: body.minX, y: body.maxY - headH, width: body.width, height: headH))
ctx.restoreGState()

// 머리띠의 고리 두 개
for x in [body.minX + 230, body.maxX - 230] {
    let ring = CGPath(roundedRect: CGRect(x: x - 22, y: body.maxY - 70, width: 44, height: 110), cornerWidth: 22, cornerHeight: 22, transform: nil)
    ctx.addPath(ring); ctx.setFillColor(rgb(0xf2f2f2)); ctx.fillPath()
}

// 달력 격자 7×5
let grid = CGRect(x: body.minX + 70, y: body.minY + 70, width: body.width - 140, height: body.height - headH - 130)
let cols = 7, rows = 5
let cw = grid.width / CGFloat(cols), ch = grid.height / CGFloat(rows)
ctx.setStrokeColor(rgb(0x4a4a4e)); ctx.setLineWidth(6)
for c in 0...cols { ctx.move(to: CGPoint(x: grid.minX + CGFloat(c) * cw, y: grid.minY)); ctx.addLine(to: CGPoint(x: grid.minX + CGFloat(c) * cw, y: grid.maxY)) }
for r in 0...rows { ctx.move(to: CGPoint(x: grid.minX, y: grid.minY + CGFloat(r) * ch)); ctx.addLine(to: CGPoint(x: grid.maxX, y: grid.minY + CGFloat(r) * ch)) }
ctx.strokePath()

// 일정 막대(캘린더 색)
func bar(_ c: Int, _ r: Int, _ color: UInt32, _ line: Int = 0, span: Int = 1) {
    let x = grid.minX + CGFloat(c) * cw + 12
    let y = grid.maxY - CGFloat(r + 1) * ch + ch - 52 - CGFloat(line) * 30
    ctx.setFillColor(rgb(color)); ctx.fill(CGRect(x: x, y: y, width: cw * CGFloat(span) - 24, height: 24))
}
bar(1, 1, 0xb8875a, span: 3)
bar(5, 0, 0xa8d8d8)
bar(3, 2, 0xb8875a)
bar(2, 3, 0xe0a060)
bar(6, 1, 0xa8d8d8, 1)
bar(4, 4, 0xe0a060)

// 주말 칸 머리(빨간 점)
ctx.setFillColor(rgb(0xff6b6b))
for r in 0..<rows {
    for c in [0, 6] {
        ctx.fillEllipse(in: CGRect(x: grid.minX + CGFloat(c) * cw + 16, y: grid.maxY - CGFloat(r) * ch - 34, width: 18, height: 18))
    }
}

// 오늘 칸: 흰 테두리
let today = CGRect(x: grid.minX + 3 * cw, y: grid.maxY - 4 * ch, width: cw, height: ch) // 넷째 주 수요일(빈 칸)
ctx.setStrokeColor(rgb(0xffffff)); ctx.setLineWidth(12); ctx.stroke(today.insetBy(dx: 3, dy: 3))

NSGraphicsContext.current = nil
try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: out))
