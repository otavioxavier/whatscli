/*
 *BSD 3-Clause License
 *
 *Copyright (c) 2017, Baozisoftware
 *All rights reserved.
 *
 *Redistribution and use in source and binary forms, with or without
 *modification, are permitted provided that the following conditions are met:
 *
 ** Redistributions of source code must retain the above copyright notice, this
 *  list of conditions and the following disclaimer.
 *
 ** Redistributions in binary form must reproduce the above copyright notice,
 *  this list of conditions and the following disclaimer in the documentation
 *  and/or other materials provided with the distribution.
 *
 ** Neither the name of the copyright holder nor the names of its
 *  contributors may be used to endorse or promote products derived from
 *  this software without specific prior written permission.
 *
 *THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
 *AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 *IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 *DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
 *FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 *DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 *SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
 *CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
 *OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 *OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package qrcode

import (
	nbytes "bytes"
	"fmt"
	"image/png"
	"io"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/skip2/go-qrcode"
)

// ANSI colors used to draw the QR code. Two separate SGR sequences are used
// for mixed cells because tview's ANSIWriter stops parsing an SGR sequence at
// the first 38/48 color selector.
const (
	ansiDark  = "\033[38;5;0m" // black foreground
	ansiLight = "\033[48;5;7m" // light background
	ansiReset = "\033[0m"
)

const (
	// terminalQuietZone is the border (in modules) kept around the symbol.
	// The QR library already provides four modules; we crop those and add a
	// smaller border so the code stays compact in a terminal.
	terminalQuietZone = 2

	// The QR code never takes more than these fractions of the available
	// terminal panel. This keeps it readable without dominating the screen.
	maxWidthRatio  = 0.62
	maxHeightRatio = 0.70
	maxScale       = 2
)

type consoleColor string
type consoleColors struct {
	NormalBlack   consoleColor
	NormalRed     consoleColor
	NormalGreen   consoleColor
	NormalYellow  consoleColor
	NormalBlue    consoleColor
	NormalMagenta consoleColor
	NormalCyan    consoleColor
	NormalWhite   consoleColor
	BrightBlack   consoleColor
	BrightRed     consoleColor
	BrightGreen   consoleColor
	BrightYellow  consoleColor
	BrightBlue    consoleColor
	BrightMagenta consoleColor
	BrightCyan    consoleColor
	BrightWhite   consoleColor
}
type qrcodeRecoveryLevel qrcode.RecoveryLevel
type qrcodeRecoveryLevels struct {
	Low     qrcodeRecoveryLevel
	Medium  qrcodeRecoveryLevel
	High    qrcodeRecoveryLevel
	Highest qrcodeRecoveryLevel
}

var (
	ConsoleColors consoleColors = consoleColors{
		NormalBlack:   "\033[38;5;0m  \033[0m",
		NormalRed:     "\033[38;5;1m  \033[0m",
		NormalGreen:   "\033[38;5;2m  \033[0m",
		NormalYellow:  "\033[38;5;3m  \033[0m",
		NormalBlue:    "\033[38;5;4m  \033[0m",
		NormalMagenta: "\033[38;5;5m  \033[0m",
		NormalCyan:    "\033[38;5;6m  \033[0m",
		NormalWhite:   "\033[38;5;7m  \033[0m",
		BrightBlack:   "\033[48;5;0m  \033[0m",
		BrightRed:     "\033[48;5;1m  \033[0m",
		BrightGreen:   "\033[48;5;2m  \033[0m",
		BrightYellow:  "\033[48;5;3m  \033[0m",
		BrightBlue:    "\033[48;5;4m  \033[0m",
		BrightMagenta: "\033[48;5;5m  \033[0m",
		BrightCyan:    "\033[48;5;6m  \033[0m",
		BrightWhite:   "\033[48;5;7m  \033[0m"}
	QRCodeRecoveryLevels = qrcodeRecoveryLevels{
		Low:     qrcodeRecoveryLevel(qrcode.Low),
		Medium:  qrcodeRecoveryLevel(qrcode.Medium),
		High:    qrcodeRecoveryLevel(qrcode.High),
		Highest: qrcodeRecoveryLevel(qrcode.Highest)}
)

type QRCodeString string

func (v *QRCodeString) Print() {
	fmt.Fprintln(outer, *v)
}

type qrcodeTerminal struct {
	front consoleColor
	back  consoleColor
	level qrcodeRecoveryLevel
}

// Get renders the code at its compact size (one terminal cell per module).
func (v *qrcodeTerminal) Get(content interface{}) *QRCodeString {
	return v.GetFitted(content, 0, 0)
}

// GetFitted renders the code so it fits the given terminal panel (in cells).
// It uses half-block characters (two QR rows per terminal row) and picks the
// largest integer scale that still leaves room for the rest of the UI.
func (v *qrcodeTerminal) GetFitted(content interface{}, maxWidth, maxHeight int) *QRCodeString {
	modules, ok := v.symbol(content, v.level)
	if !ok {
		return nil
	}

	base := len(modules) + 2*terminalQuietZone
	baseRows := (base + 1) / 2

	// On very small terminals, prefer a lower error-correction level: that
	// produces a smaller symbol instead of distorting the QR code.
	if maxWidth > 0 && maxHeight > 0 && (base > maxWidth || baseRows > maxHeight) {
		if smaller, ok := v.symbol(content, QRCodeRecoveryLevels.Low); ok {
			smallBase := len(smaller) + 2*terminalQuietZone
			if smallBase < base {
				modules = smaller
				base = smallBase
				baseRows = (base + 1) / 2
			}
		}
	}

	scale := 1
	if maxWidth > 0 && maxHeight > 0 {
		limitW := int(float64(maxWidth) * maxWidthRatio)
		limitH := int(float64(maxHeight) * maxHeightRatio)
		if limitW < base {
			limitW = base
		}
		if limitH < baseRows {
			limitH = baseRows
		}
		scale = limitW / base
		if heightScale := limitH / baseRows; heightScale < scale {
			scale = heightScale
		}
		if scale > maxScale {
			scale = maxScale
		}
		if scale < 1 {
			scale = 1
		}
	}

	size := base * scale
	if maxWidth > 0 && maxHeight > 0 &&
		(float64(size) > float64(maxWidth)*maxWidthRatio ||
			float64(baseRows*scale) > float64(maxHeight)*maxHeightRatio) {
		// Keep the code compact: Braille dots store 2x4 modules per cell,
		// roughly halving both dimensions compared to half-block characters.
		brailleWidth := (base + 1) / 2
		brailleHeight := (base + 3) / 4
		if brailleWidth <= maxWidth && brailleHeight <= maxHeight {
			return v.renderBraille(modules, maxWidth)
		}
	}

	return v.render(modules, scale, maxWidth)
}

func (v *qrcodeTerminal) symbol(content interface{}, level qrcodeRecoveryLevel) ([][]bool, bool) {
	var qr *qrcode.QRCode
	var err error
	switch t := content.(type) {
	case string:
		qr, err = qrcode.New(t, qrcode.RecoveryLevel(level))
	case []byte:
		qr, err = qrcode.New(string(t), qrcode.RecoveryLevel(level))
	}
	if err != nil || qr == nil {
		return nil, false
	}
	return cropQuietZone(qr.Bitmap()), true
}

// cropQuietZone removes the four-module border added by go-qrcode.
func cropQuietZone(bitmap [][]bool) [][]bool {
	const libraryQuietZone = 4
	if len(bitmap) <= 2*libraryQuietZone {
		return bitmap
	}
	size := len(bitmap) - 2*libraryQuietZone
	modules := make([][]bool, size)
	for y := 0; y < size; y++ {
		modules[y] = make([]bool, size)
		copy(modules[y], bitmap[y+libraryQuietZone][libraryQuietZone:libraryQuietZone+size])
	}
	return modules
}

// moduleGrid expands the QR modules by scale and adds the quiet zone.
func moduleGrid(modules [][]bool, scale int) [][]bool {
	quiet := terminalQuietZone * scale
	size := len(modules)*scale + 2*quiet
	grid := make([][]bool, size)
	for y := range grid {
		grid[y] = make([]bool, size)
	}
	for y, row := range modules {
		for x, dark := range row {
			if !dark {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					grid[quiet+y*scale+dy][quiet+x*scale+dx] = true
				}
			}
		}
	}
	return grid
}

// render expands the modules by scale, packs two module rows into each
// terminal row using half-block characters and centers the result horizontally.
func (v *qrcodeTerminal) render(modules [][]bool, scale, viewportWidth int) *QRCodeString {
	grid := moduleGrid(modules, scale)
	size := len(grid)

	padding := 0
	if viewportWidth > size {
		padding = (viewportWidth - size) / 2
	}
	prefix := strings.Repeat(" ", padding)

	lines := make([]string, 0, (size+1)/2)
	for y := 0; y < size; y += 2 {
		var line strings.Builder
		line.WriteString(prefix)
		for x := 0; x < size; {
			char, colors := qrCell(grid[y][x], y+1 < size && grid[y+1][x])
			run := 1
			for x+run < size {
				nextChar, nextColors := qrCell(grid[y][x+run], y+1 < size && grid[y+1][x+run])
				if nextColors != colors || nextChar != char {
					break
				}
				run++
			}
			line.WriteString(colors)
			line.WriteString(strings.Repeat(char, run))
			line.WriteString(ansiReset)
			x += run
		}
		lines = append(lines, line.String())
	}

	result := QRCodeString(strings.Join(lines, "\n"))
	return &result
}

// qrCell maps a pair of module rows to one terminal cell.
func qrCell(top, bottom bool) (string, string) {
	switch {
	case top && bottom:
		return "█", ansiDark
	case top:
		return "▀", ansiDark + ansiLight
	case bottom:
		return "▄", ansiDark + ansiLight
	default:
		return " ", ansiLight
	}
}

// renderBraille packs a 2x4 module block into one Braille character. The
// dots are the four rows (1,2,3,7 on the left and 4,5,6,8 on the right).
// This mode is only used when even the compact half-block rendering would
// not fit in the terminal window.
func (v *qrcodeTerminal) renderBraille(modules [][]bool, viewportWidth int) *QRCodeString {
	grid := moduleGrid(modules, 1)
	size := len(grid)
	cellsWide := (size + 1) / 2
	cellsHigh := (size + 3) / 4

	padding := 0
	if viewportWidth > cellsWide {
		padding = (viewportWidth - cellsWide) / 2
	}
	prefix := strings.Repeat(" ", padding)

	dot := func(x, y int) bool {
		return x < size && y < size && grid[y][x]
	}

	lines := make([]string, 0, cellsHigh)
	for cy := 0; cy < cellsHigh; cy++ {
		var line strings.Builder
		line.WriteString(prefix)
		for cx := 0; cx < cellsWide; cx++ {
			x, y := 2*cx, 4*cy
			bits := 0
			if dot(x, y) {
				bits |= 1 << 0
			}
			if dot(x, y+1) {
				bits |= 1 << 1
			}
			if dot(x, y+2) {
				bits |= 1 << 2
			}
			if dot(x+1, y) {
				bits |= 1 << 3
			}
			if dot(x+1, y+1) {
				bits |= 1 << 4
			}
			if dot(x+1, y+2) {
				bits |= 1 << 5
			}
			if dot(x, y+3) {
				bits |= 1 << 6
			}
			if dot(x+1, y+3) {
				bits |= 1 << 7
			}
			line.WriteString(ansiDark)
			line.WriteString(ansiLight)
			line.WriteRune(rune(0x2800 + bits))
			line.WriteString(ansiReset)
		}
		lines = append(lines, line.String())
	}

	result := QRCodeString(strings.Join(lines, "\n"))
	return &result
}

func (v *qrcodeTerminal) Get2(bytes []byte) *QRCodeString {
	data, err := parseQR(bytes)
	if err != nil {
		return nil
	}
	return v.render(cropQuietZone(data), 1, 0)
}

func New2(front, back consoleColor, level qrcodeRecoveryLevel) *qrcodeTerminal {
	obj := qrcodeTerminal{front: front, back: back, level: level}
	return &obj
}

func New() *qrcodeTerminal {
	front, back, level := ConsoleColors.BrightBlack, ConsoleColors.BrightWhite, QRCodeRecoveryLevels.Medium
	return New2(front, back, level)
}

func parseQR(bytes []byte) (data [][]bool, err error) {
	r := nbytes.NewReader(bytes)
	img, err := png.Decode(r)
	if err == nil {
		rect := img.Bounds()
		mx, my := rect.Max.X, rect.Max.Y
		data = make([][]bool, my)
		for y := 0; y < my; y++ {
			data[y] = make([]bool, mx)
			for x := 0; x < mx; x++ {
				c := img.At(x, y)
				r, _, _, _ := c.RGBA()
				data[y][x] = r == 0
			}
		}
	}
	return
}

func (_ *qrcodeTerminal) SetOutput(out io.Writer) {
	outer = out
}

var outer = colorable.NewColorableStdout()
