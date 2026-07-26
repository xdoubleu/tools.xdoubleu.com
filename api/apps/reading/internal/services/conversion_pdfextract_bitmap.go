package services

import (
	"image"
	"image/color"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

// Byte layout constants for the pixel formats FPDFImageObj_GetBitmap can
// return: how many bytes each pixel occupies, and the maximum value of a
// color/alpha byte (used to synthesize full opacity for formats with no
// alpha channel).
const (
	bytesPerPixelBGRA = 4
	bytesPerPixelBGRX = 4
	bytesPerPixelBGR  = 3
	maxColorByte      = 255
)

// bitmapToImage reads an FPDF_BITMAP's raw pixel buffer and builds a Go
// image.Image. Uses the bitmap (not FPDFImageObj_GetImageDataDecoded) so the
// encoded-filter/colorspace zoo (JPX, CCITT, indexed palettes) is sidestepped
// entirely — PDFium already decoded it to raw pixels for us.
func bitmapToImage(
	instance pdfium.Pdfium,
	bitmap references.FPDF_BITMAP,
) (image.Image, error) {
	widthResp, err := instance.FPDFBitmap_GetWidth(
		&requests.FPDFBitmap_GetWidth{Bitmap: bitmap},
	)
	if err != nil {
		return nil, err
	}
	heightResp, err := instance.FPDFBitmap_GetHeight(
		&requests.FPDFBitmap_GetHeight{Bitmap: bitmap},
	)
	if err != nil {
		return nil, err
	}
	strideResp, err := instance.FPDFBitmap_GetStride(
		&requests.FPDFBitmap_GetStride{Bitmap: bitmap},
	)
	if err != nil {
		return nil, err
	}
	formatResp, err := instance.FPDFBitmap_GetFormat(
		&requests.FPDFBitmap_GetFormat{Bitmap: bitmap},
	)
	if err != nil {
		return nil, err
	}
	bufResp, err := instance.FPDFBitmap_GetBuffer(
		&requests.FPDFBitmap_GetBuffer{Bitmap: bitmap},
	)
	if err != nil {
		return nil, err
	}

	width, height, stride := widthResp.Width, heightResp.Height, strideResp.Stride
	buf := bufResp.Buffer

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		rowOff := y * stride
		for x := range width {
			r, g, b, a := pixelAt(buf, rowOff, x, formatResp.Format)
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: a})
		}
	}
	return img, nil
}

// pixelAt reads pixel (x within a row starting at rowOff) from a raw bitmap
// buffer in the given PDFium format, returning transparent black for any
// out-of-bounds read (a corrupt/truncated buffer must not panic) and opaque
// black for a format PDFium hasn't reported before (FPDF_BITMAP_FORMAT_GRAY
// is the only single-channel format it returns; anything else unrecognized
// falls here alongside FPDF_BITMAP_FORMAT_UNKNOWN).
func pixelAt(
	buf []byte,
	rowOff, x int,
	format enums.FPDF_BITMAP_FORMAT,
) (byte, byte, byte, byte) {
	switch format {
	case enums.FPDF_BITMAP_FORMAT_BGRA:
		off := rowOff + x*bytesPerPixelBGRA
		if off+3 >= len(buf) {
			return 0, 0, 0, 0
		}
		return buf[off+2], buf[off+1], buf[off], buf[off+3]
	case enums.FPDF_BITMAP_FORMAT_BGRX:
		off := rowOff + x*bytesPerPixelBGRX
		if off+2 >= len(buf) {
			return 0, 0, 0, 0
		}
		return buf[off+2], buf[off+1], buf[off], maxColorByte
	case enums.FPDF_BITMAP_FORMAT_BGR:
		off := rowOff + x*bytesPerPixelBGR
		if off+2 >= len(buf) {
			return 0, 0, 0, 0
		}
		return buf[off+2], buf[off+1], buf[off], maxColorByte
	case enums.FPDF_BITMAP_FORMAT_GRAY:
		if rowOff+x >= len(buf) {
			return 0, 0, 0, 0
		}
		v := buf[rowOff+x]
		return v, v, v, maxColorByte
	case enums.FPDF_BITMAP_FORMAT_UNKNOWN:
		return 0, 0, 0, maxColorByte
	default:
		return 0, 0, 0, maxColorByte
	}
}
