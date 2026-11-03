// Pulls one labeled image per screen out of a phone recording of an NYT Crossplay
// game's coach view, so that the moves can be transcribed. Step through the game
// while recording, pausing for about a second on each turn.
//
// usage: swift -O scripts/crossplay/extract_frames.swift <video> <outdir>
//
// A new image starts whenever the screen settles on something different. Each one
// is cropped to the header, board and tray, with A-O / 1-15 axes and a faint grid
// added, and the turn chips below. Keep the output out of the repo: it shows the
// opponent's name.
//
// The crop positions were measured on a 1206x2622 recording. They scale with the
// video's width, but another phone's layout may differ, so check that the grid
// lines up on one image.
import AVFoundation
import AppKit

// How fast to sample, and what counts as changed: the mean difference per
// pixel, out of 255, between samples.
let samplesPerSecond = 10.0
let settledBelow = 0.6
let minSettledSeconds = 0.25
let sameScreenBelow = 1.5

let args = CommandLine.arguments
guard args.count == 3 else {
  print("usage: extract_frames.swift <video> <outdir>")
  exit(2)
}
let out = URL(fileURLWithPath: args[2])
try FileManager.default.createDirectory(at: out, withIntermediateDirectories: true)

func wait<T>(_ work: @escaping () async throws -> T) throws -> T {
  var result: Result<T, Error>!
  let done = DispatchSemaphore(value: 0)
  Task {
    do { result = .success(try await work()) } catch { result = .failure(error) }
    done.signal()
  }
  done.wait()
  return try result.get()
}

let asset = AVURLAsset(url: URL(fileURLWithPath: args[1]))
let track = try wait { try await asset.loadTracks(withMediaType: .video)[0] }
let size = try wait { try await track.load(.naturalSize) }
print("video \(Int(size.width))x\(Int(size.height))")
if Int(size.width) != 1206 || Int(size.height) != 2622 {
  print("WARNING: crops were measured on 1206x2622; check that the grid lines up")
}

// First pass: a small grayscale thumbnail of each sample, without the status bar,
// whose clock and icons change on their own.
struct Sample {
  let time: Double
  let pixels: [UInt8]
}
let reader = try AVAssetReader(asset: asset)
let output = AVAssetReaderTrackOutput(
  track: track,
  outputSettings: [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA])
output.alwaysCopiesSampleData = false
reader.add(output)
reader.startReading()

var samples: [Sample] = []
var nextTime = 0.0
while let buffer = output.copyNextSampleBuffer() {
  let time = CMSampleBufferGetPresentationTimeStamp(buffer).seconds
  guard time >= nextTime, let image = CMSampleBufferGetImageBuffer(buffer) else { continue }
  nextTime = time + 1 / samplesPerSecond
  CVPixelBufferLockBaseAddress(image, .readOnly)
  let width = CVPixelBufferGetWidth(image)
  let height = CVPixelBufferGetHeight(image)
  let rowBytes = CVPixelBufferGetBytesPerRow(image)
  let base = CVPixelBufferGetBaseAddress(image)!.assumingMemoryBound(to: UInt8.self)
  let step = max(width / 180, 1)
  var pixels: [UInt8] = []
  for y in stride(from: height * 7 / 100, to: height, by: step) {
    for x in stride(from: 0, to: width, by: step) {
      pixels.append(base[y * rowBytes + x * 4 + 1])
    }
  }
  CVPixelBufferUnlockBaseAddress(image, .readOnly)
  samples.append(Sample(time: time, pixels: pixels))
}
guard reader.status == .completed, !samples.isEmpty else {
  print("couldn't read the video: \(String(describing: reader.error))")
  exit(1)
}

func difference(_ a: [UInt8], _ b: [UInt8]) -> Double {
  var total = 0
  for i in 0..<a.count { total += abs(Int(a[i]) - Int(b[i])) }
  return Double(total) / Double(a.count)
}

// Find the stretches where the screen holds still, merging neighbors that show
// the same thing.
var screens: [(first: Int, last: Int)] = []
var start = 0
for i in 1...samples.count {
  if i < samples.count && difference(samples[i].pixels, samples[i - 1].pixels) <= settledBelow {
    continue
  }
  if samples[i - 1].time - samples[start].time >= minSettledSeconds {
    if let prev = screens.last,
      difference(samples[prev.last].pixels, samples[i - 1].pixels) < sameScreenBelow
    {
      screens[screens.count - 1].last = i - 1
    } else {
      screens.append((start, i - 1))
    }
  }
  start = i
}

// Second pass: render the middle of each stretch.
func annotate(_ frame: CGImage) -> CGImage {
  let scale = Double(frame.width) / 1206
  let px = { (v: Double) in Int(v * scale) }
  let margin = px(56)
  let (cropY, cropHeight) = (px(340), px(1480))
  let (stripY, stripHeight) = (px(2150), px(210))
  let (boardX, boardY, cell) = (62 * scale, 522 * scale, 72.3 * scale)
  let (width, height) = (frame.width + margin, cropHeight + stripHeight + margin)

  let ctx = CGContext(
    data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
    space: CGColorSpaceCreateDeviceRGB(),
    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
  ctx.setFillColor(.white)
  ctx.fill(CGRect(x: 0, y: 0, width: width, height: height))
  ctx.draw(
    frame.cropping(to: CGRect(x: 0, y: cropY, width: frame.width, height: cropHeight))!,
    in: CGRect(x: margin, y: stripHeight, width: frame.width, height: cropHeight))
  ctx.draw(
    frame.cropping(to: CGRect(x: 0, y: stripY, width: frame.width, height: stripHeight))!,
    in: CGRect(x: margin, y: 0, width: frame.width, height: stripHeight))

  // Core Graphics counts up from the bottom.
  let top = Double(height - margin) - (boardY - Double(cropY))
  let left = Double(margin) + boardX
  ctx.setStrokeColor(CGColor(red: 1, green: 0, blue: 0.6, alpha: 0.45))
  ctx.setLineWidth(1)
  for i in 0...15 {
    let d = Double(i) * cell
    ctx.move(to: CGPoint(x: left + d, y: top))
    ctx.addLine(to: CGPoint(x: left + d, y: top - 15 * cell))
    ctx.move(to: CGPoint(x: left, y: top - d))
    ctx.addLine(to: CGPoint(x: left + 15 * cell, y: top - d))
  }
  ctx.strokePath()

  NSGraphicsContext.current = NSGraphicsContext(cgContext: ctx, flipped: false)
  let font: [NSAttributedString.Key: Any] = [
    .font: NSFont.boldSystemFont(ofSize: 30 * scale),
    .foregroundColor: NSColor(red: 0.8, green: 0, blue: 0.45, alpha: 1),
  ]
  for i in 0..<15 {
    let column = String(UnicodeScalar(UInt8(65 + i))) as NSString
    let columnSize = column.size(withAttributes: font)
    column.draw(
      at: NSPoint(
        x: left + (Double(i) + 0.5) * cell - columnSize.width / 2,
        y: Double(height - margin) + 8 * scale),
      withAttributes: font)
    let row = "\(i + 1)" as NSString
    let rowSize = row.size(withAttributes: font)
    row.draw(
      at: NSPoint(
        x: Double(margin) - rowSize.width - 4 * scale,
        y: top - (Double(i) + 0.5) * cell - rowSize.height / 2),
      withAttributes: font)
  }
  return ctx.makeImage()!
}

let generator = AVAssetImageGenerator(asset: asset)
generator.requestedTimeToleranceBefore = .zero
generator.requestedTimeToleranceAfter = .zero
generator.appliesPreferredTrackTransform = true

for (n, screen) in screens.enumerated() {
  let time = (samples[screen.first].time + samples[screen.last].time) / 2
  let frame = try wait {
    try await generator.image(at: CMTime(seconds: time, preferredTimescale: 600)).image
  }
  let path = out.appendingPathComponent(String(format: "frame%03d.png", n + 1))
  let dest = CGImageDestinationCreateWithURL(path as CFURL, "public.png" as CFString, 1, nil)!
  CGImageDestinationAddImage(dest, annotate(frame), nil)
  guard CGImageDestinationFinalize(dest) else {
    print("couldn't write \(path.path)")
    exit(1)
  }
}
print("wrote \(screens.count) images to \(out.path)")
