import Foundation
import Vision
import AppKit
import PDFKit

let args = CommandLine.arguments
guard args.count > 1 else {
    print("Usage: doc_vision_ocr <file_path>")
    exit(1)
}

let filePath = args[1]
let url = URL(fileURLWithPath: filePath)

func ocr(cgImage: CGImage) {
    let request = VNRecognizeTextRequest { (req, err) in
        guard let obs = req.results as? [VNRecognizedTextObservation] else { return }
        for o in obs {
            if let c = o.topCandidates(1).first {
                print(c.string)
            }
        }
    }
    request.recognitionLevel = .accurate
    request.recognitionLanguages = ["zh-Hans", "zh-Hant", "en-US"]
    request.usesLanguageCorrection = true
    let handler = VNImageRequestHandler(cgImage: cgImage, options: [:])
    try? handler.perform([request])
}

let ext = url.pathExtension.lowercased()

if ext == "pdf" {
    if let pdf = PDFDocument(url: url) {
        let maxPages = min(pdf.pageCount, 10)
        for i in 0..<maxPages {
            if let page = pdf.page(at: i) {
                let rect = page.bounds(for: .mediaBox)
                let scale: CGFloat = 2.0
                let width = max(1, Int(rect.width * scale))
                let height = max(1, Int(rect.height * scale))
                let colorSpace = CGColorSpaceCreateDeviceRGB()
                if let ctx = CGContext(
                    data: nil,
                    width: width,
                    height: height,
                    bitsPerComponent: 8,
                    bytesPerRow: 0,
                    space: colorSpace,
                    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue
                ) {
                    ctx.scaleBy(x: scale, y: scale)
                    ctx.setFillColor(NSColor.white.cgColor)
                    ctx.fill(rect)
                    page.draw(with: .mediaBox, to: ctx)
                    if let cgImage = ctx.makeImage() {
                        ocr(cgImage: cgImage)
                    }
                }
            }
        }
    }
} else {
    // 图片格式识别 (JPG, PNG, JPEG, BMP, WEBP, TIFF)
    if let nsImg = NSImage(contentsOf: url),
       let cgImg = nsImg.cgImage(forProposedRect: nil, context: nil, hints: nil) {
        ocr(cgImage: cgImg)
    }
}
