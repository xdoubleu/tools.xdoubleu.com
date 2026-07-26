package services

import (
	"image"
	"image/color"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
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

func pixelAt(
	buf []byte,
	rowOff, x int,
	format enums.FPDF_BITMAP_FORMAT,
) (r, g, b, a byte) {
	switch format {
	case enums.FPDF_BITMAP_FORMAT_BGRA:
		off := rowOff + x*4
		if off+3 >= len(buf) {
			return 0, 0, 0, 0
		}
		return buf[off+2], buf[off+1], buf[off], buf[off+3]
	case enums.FPDF_BITMAP_FORMAT_BGRX:
		off := rowOff + x*4
		if off+2 >= len(buf) {
			return 0, 0, 0, 0
		}
		return buf[off+2], buf[off+1], buf[off], 255
	case enums.FPDF_BITMAP_FORMAT_BGR:
		off := rowOff + x*3
		if off+2 >= len(buf) {
			return 0, 0, 0, 0
		}
		return buf[off+2], buf[off+1], buf[off], 255
	case enums.FPDF_BITMAP_FORMAT_GRAY:
		if rowOff+x >= len(buf) {
			return 0, 0, 0, 0
		}
		v := buf[rowOff+x]
		return v, v, v, 255
	default:
		return 0, 0, 0, 255
	}
}
