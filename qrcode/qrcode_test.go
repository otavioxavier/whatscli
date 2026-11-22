package qrcode

import (
	"regexp"
	"strings"
	"testing"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func visibleLine(line string) string {
	return ansiEscape.ReplaceAllString(line, "")
}

// sampleCode approximates the length and shape of a WhatsApp companion QR.
const sampleCode = "2@ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz,AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func TestFittedQRCode(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
	}{
		{name: "tiny terminal", width: 50, height: 20},
		{name: "default terminal", width: 80, height: 24},
		{name: "large terminal", width: 200, height: 60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code := New().GetFitted(sampleCode, tc.width, tc.height)
			if code == nil {
				t.Fatal("failed to render QR code")
			}
			lines := strings.Split(string(*code), "\n")
			for i, line := range lines {
				if w := len([]rune(visibleLine(line))); w > tc.width {
					t.Errorf("line %d is %d cells wide, viewport is %d", i, w, tc.width)
				}
			}
			if len(lines) > tc.height {
				t.Errorf("QR code is %d rows tall, viewport is %d", len(lines), tc.height)
			}
			t.Logf("viewport %dx%d -> rendered up to %d x %d cells", tc.width, tc.height, len([]rune(visibleLine(lines[0]))), len(lines))
		})
	}
}

func TestHalfBlocksPreserveModules(t *testing.T) {
	const viewportWidth = 120
	terminal := New()
	code := terminal.GetFitted(sampleCode, viewportWidth, 40)
	if code == nil {
		t.Fatal("failed to render QR code")
	}

	lines := strings.Split(string(*code), "\n")
	size := 2*len(lines) - 1 // half-block rows: (size+1)/2

	// The renderer may drop to a lower error-correction level on small
	// viewports, so pick the symbol with the matching dimensions.
	var modules [][]bool
	for _, level := range []qrcodeRecoveryLevel{QRCodeRecoveryLevels.Medium, QRCodeRecoveryLevels.Low} {
		candidate, ok := terminal.symbol(sampleCode, level)
		if ok && len(candidate)+2*terminalQuietZone == size {
			modules = candidate
			break
		}
	}
	if modules == nil {
		t.Fatalf("no symbol matches rendered size %d", size)
	}

	decoded := make([][]bool, size)
	for y := range decoded {
		decoded[y] = make([]bool, size)
	}

	padding := (viewportWidth - size) / 2
	for y, line := range lines {
		runes := []rune(visibleLine(line))
		for x, r := range runes[padding:] {
			if x >= size {
				break
			}
			switch r {
			case '█':
				decoded[2*y][x] = true
				if 2*y+1 < size {
					decoded[2*y+1][x] = true
				}
			case '▀':
				decoded[2*y][x] = true
			case '▄':
				if 2*y+1 < size {
					decoded[2*y+1][x] = true
				}
			}
		}
	}

	for y, row := range modules {
		for x, dark := range row {
			if got := decoded[y+terminalQuietZone][x+terminalQuietZone]; got != dark {
				t.Fatalf("module (%d,%d) changed during rendering: got %v, want %v", x, y, got, dark)
			}
		}
	}
}

func TestBrailleFallbackPreservesModules(t *testing.T) {
	const viewportWidth = 50
	terminal := New()
	code := terminal.GetFitted(sampleCode, viewportWidth, 20)
	if code == nil {
		t.Fatal("failed to render QR code")
	}

	lines := strings.Split(string(*code), "\n")

	var size int
	var modules [][]bool
	for _, level := range []qrcodeRecoveryLevel{QRCodeRecoveryLevels.Medium, QRCodeRecoveryLevels.Low} {
		candidate, ok := terminal.symbol(sampleCode, level)
		if !ok {
			continue
		}
		candidateSize := len(candidate) + 2*terminalQuietZone
		if (candidateSize+3)/4 == len(lines) {
			modules, size = candidate, candidateSize
			break
		}
	}
	if modules == nil {
		t.Fatalf("no symbol matches %d rendered rows", len(lines))
	}

	decoded := make([][]bool, size)
	for y := range decoded {
		decoded[y] = make([]bool, size)
	}
	padding := (viewportWidth - (size+1)/2) / 2
	for cy, line := range lines {
		runes := []rune(visibleLine(line))
		for cx, r := range runes[padding:] {
			if r < 0x2800 || r > 0x28ff {
				continue
			}
			bits := int(r - 0x2800)
			x, y := 2*cx, 4*cy
			for _, dot := range []struct {
				bit    int
				dx, dy int
			}{
				{1 << 0, 0, 0}, {1 << 1, 0, 1}, {1 << 2, 0, 2}, {1 << 3, 1, 0},
				{1 << 4, 1, 1}, {1 << 5, 1, 2}, {1 << 6, 0, 3}, {1 << 7, 1, 3},
			} {
				if bits&dot.bit != 0 && x+dot.dx < size && y+dot.dy < size {
					decoded[y+dot.dy][x+dot.dx] = true
				}
			}
		}
	}
	for y, row := range modules {
		for x, dark := range row {
			if got := decoded[y+terminalQuietZone][x+terminalQuietZone]; got != dark {
				t.Fatalf("module (%d,%d) changed in Braille mode: got %v, want %v", x, y, got, dark)
			}
		}
	}
}
