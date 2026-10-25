package auth

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// qrQuietPx is the white border scanners need around the code.
const qrQuietPx = 16

func TestRenderQRSVG_WrapsSquarePNGWithQuietZone(t *testing.T) {
	//nolint:exhaustruct //other GenerateOpts fields use library defaults
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      totpIssuer,
		AccountName: "someone@example.com",
	})
	require.NoError(t, err)

	svg, err := renderQRSVG(key)
	require.NoError(t, err)

	prefix := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`+
			`<image width="%d" height="%d" href="data:image/png;base64,`,
		totpQRSize, totpQRSize, totpQRSize, totpQRSize,
	)
	const suffix = `"/></svg>`
	require.True(t, strings.HasPrefix(svg, prefix), svg[:min(len(svg), 200)])
	require.True(t, strings.HasSuffix(svg, suffix))

	raw, err := base64.StdEncoding.DecodeString(
		strings.TrimSuffix(strings.TrimPrefix(svg, prefix), suffix),
	)
	require.NoError(t, err)
	img, err := png.Decode(bytes.NewReader(raw))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, totpQRSize, totpQRSize), img.Bounds())

	darkInQuietZone, darkInCode := 0, 0
	for y := range totpQRSize {
		for x := range totpQRSize {
			r, _, _, _ := img.At(x, y).RGBA()
			if r >= 0x8000 {
				continue
			}
			inQuietZone := x < qrQuietPx || y < qrQuietPx ||
				x >= totpQRSize-qrQuietPx || y >= totpQRSize-qrQuietPx
			if inQuietZone {
				darkInQuietZone++
			} else {
				darkInCode++
			}
		}
	}
	assert.Zero(t, darkInQuietZone)
	assert.Positive(t, darkInCode)
}
