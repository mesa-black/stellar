package stellar

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	termCols = 40
	termRows = 20
	// A Braille cell holds 2x4 dots, so the canvas is 80x80 dots.
	dotsW = termCols * 2
	dotsH = termRows * 4
)

// brailleBit[y][x] is the bit of the dot at column x, row y of a Braille cell.
var brailleBit = [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

var (
	magnitudeGlyph = [Magnitudes]string{"✦", "*", "•"}
	// classANSI are the 256-colour codes of the spectral classes, from hot
	// blue to cool red. A terminal has no room for the marks the pictures
	// use, so here the class is told by colour.
	classANSI = [Classes]int{111, 231, 222, 215, 210}
)

// Text renders the constellation for a terminal: edges as Braille dots,
// main stars as glyphs, inside a frame carrying the name. Background stars
// are not drawn. With color set, ANSI escape sequences are used.
func (c *Constellation) Text(color bool) string {
	var dots [termRows][termCols]rune
	plot := func(x, y int) { dots[y/4][x/2] |= brailleBit[y%4][x%2] }

	for _, e := range c.Edges {
		ax, ay := starDot(c.Stars[e.A])
		bx, by := starDot(c.Stars[e.B])
		line(ax, ay, bx, by, plot)
	}

	glyphs := map[[2]int]Star{}
	for _, s := range c.Stars {
		x, y := starDot(s)
		glyphs[[2]int{y / 4, x / 2}] = s
	}

	var b strings.Builder
	b.WriteString(border("┌", "┐", fmt.Sprintf("stellar v%d", Version)))
	for row := 0; row < termRows; row++ {
		b.WriteString("│")
		for col := 0; col < termCols; col++ {
			if s, ok := glyphs[[2]int{row, col}]; ok {
				g := magnitudeGlyph[s.Magnitude]
				if color {
					g = fmt.Sprintf("\x1b[1;38;5;%dm%s\x1b[0m", classANSI[s.Class], g)
				}
				b.WriteString(g)
				continue
			}
			if dots[row][col] == 0 {
				b.WriteByte(' ')
				continue
			}
			cell := string(0x2800 + dots[row][col])
			if color {
				cell = "\x1b[38;5;61m" + cell + "\x1b[0m"
			}
			b.WriteString(cell)
		}
		b.WriteString("│\n")
	}
	b.WriteString(border("└", "┘", c.Name))
	return b.String()
}

// starDot maps a main star to dot coordinates on the Braille canvas.
func starDot(s Star) (x, y int) {
	return 4 + s.X*(dotsW-8)/StarGrid, 4 + s.Y*(dotsH-8)/StarGrid
}

func border(left, right, label string) string {
	label = "─[ " + label + " ]"
	fill := termCols - utf8.RuneCountInString(label)
	return left + label + strings.Repeat("─", fill) + right + "\n"
}

// line walks the Bresenham line from (x0, y0) to (x1, y1).
func line(x0, y0, x1, y1 int, plot func(x, y int)) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	for err := dx + dy; ; {
		plot(x0, y0)
		if x0 == x1 && y0 == y1 {
			return
		}
		if e2 := 2 * err; e2 >= dy {
			err += dy
			x0 += sx
		} else {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
