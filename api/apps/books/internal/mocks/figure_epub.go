package mocks

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
)

// figurePNGSide is above the figure analysis' minimum size.
const figurePNGSide = 60

// FigureEPUB returns a one-chapter EPUB showing figures distinct figure-sized
// PNGs beside textChars characters of body text.
func FigureEPUB(figures, textChars int) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name string, data []byte) {
		w, _ := zw.Create(name)
		_, _ = w.Write(data)
	}

	var manifest, imgs strings.Builder
	for i := range figures {
		name := fmt.Sprintf("fig%d.png", i)
		fmt.Fprintf(&manifest, `<item id="f%d" href="%s" media-type="image/png"/>`, i, name)
		fmt.Fprintf(&imgs, `<img src="%s"/>`, name)
		write("OEBPS/"+name, distinctPNG(i))
	}

	write("mimetype", []byte("application/epub+zip"))
	write("META-INF/container.xml", []byte(`<?xml version="1.0"?>`+
		`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container"`+
		` version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf"`+
		` media-type="application/oebps-package+xml"/></rootfiles></container>`))
	write("OEBPS/content.opf", []byte(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
<manifest><item id="ch" href="ch.xhtml" media-type="application/xhtml+xml"/>`+
		manifest.String()+`</manifest>
<spine><itemref idref="ch"/></spine>
</package>`))
	write("OEBPS/ch.xhtml", []byte(xhtmlHead+"<body><p>"+strings.Repeat("x", textChars)+
		"</p>"+imgs.String()+"</body></html>"))
	_ = zw.Close()
	return buf.Bytes()
}

func distinctPNG(seed int) []byte {
	img := image.NewGray(image.Rect(0, 0, figurePNGSide, figurePNGSide))
	img.SetGray(0, 0, color.Gray{Y: uint8(seed)}) //nolint:gosec // test seeds are small
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
