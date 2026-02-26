package image

import (
	"bytes"
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/abdullahdiaa/garabic"
	"github.com/fogleman/gg"
)

type Generator struct {
	fontDir string
}

func NewGenerator(fontDir string) *Generator {
	return &Generator{
		fontDir: fontDir,
	}
}

func (g *Generator) GenerateHadithImage(arabicText, englishText string) ([]byte, error) {
	const W, H = 1080, 1080
	dc := gg.NewContext(W, H)

	g.drawBackground(dc)

	// Arabic Text
	arabicFontPath := g.getFontPath("Amiri-Regular.ttf")
	// Load font with size 60
	if err := dc.LoadFontFace(arabicFontPath, 60); err != nil {
		return nil, fmt.Errorf("failed to load arabic font: %w", err)
	}

	dc.SetHexColor("#1a1a1a") // Dark charcoal

	// Shape Arabic text
	shapedArabic := garabic.Shape(arabicText)

	// Wrap text
	maxWidth := float64(W) - 160 // 80px padding on each side
	lines := dc.WordWrap(shapedArabic, maxWidth)

	// Calculate vertical positioning
	lineHeight := dc.FontHeight() * 1.5
	arabicHeight := float64(len(lines)) * lineHeight

	// We want to center everything.
	// Let's reserve top 50% for Arabic, bottom 50% for English?
	// Or just flow them with a gap.

	// Let's try to center the whole block?
	// Or put Arabic at visual center of top half.

	arabicStartY := (float64(H)/2 - arabicHeight) / 2 + 60

	for i, line := range lines {
		// Reverse line for RTL rendering in LTR engine
		reversedLine := g.reversePreservingCombiningMarks(line)
		dc.DrawStringAnchored(reversedLine, float64(W)/2, arabicStartY+float64(i)*lineHeight, 0.5, 0.5)
	}

	// English Text
	englishFontPath := g.getFontPath("Caveat-Regular.ttf")
	if err := dc.LoadFontFace(englishFontPath, 50); err != nil {
		return nil, fmt.Errorf("failed to load english font: %w", err)
	}

	englishLines := dc.WordWrap(englishText, maxWidth)
	englishHeight := float64(len(englishLines)) * lineHeight

	// Center english in bottom half
	englishStartY := float64(H)/2 + (float64(H)/2-englishHeight)/2

	for i, line := range englishLines {
		dc.DrawStringAnchored(line, float64(W)/2, englishStartY+float64(i)*lineHeight, 0.5, 0.5)
	}

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("failed to encode png: %w", err)
	}

	return buf.Bytes(), nil
}

func (g *Generator) drawBackground(dc *gg.Context) {
	// Fill background with off-white/beige
	// #FAF8F5
	dc.SetHexColor("#FAF8F5")
	dc.Clear()

	// Add noise
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	width := dc.Width()
	height := dc.Height()

	noiseCount := (width * height) / 10 // Density
	for i := 0; i < noiseCount; i++ {
		x := rnd.Float64() * float64(width)
		y := rnd.Float64() * float64(height)

		// Random gray/brown color
		gray := uint8(200 + rnd.Intn(55)) // Light noise
		alpha := uint8(10 + rnd.Intn(20)) // Low alpha

		dc.SetRGBA255(int(gray), int(gray), int(gray), int(alpha))
		dc.SetPixel(int(x), int(y))
	}
}

func (g *Generator) getFontPath(fontName string) string {
	return filepath.Join(g.fontDir, fontName)
}

func (g *Generator) reversePreservingCombiningMarks(s string) string {
	runes := []rune(s)
	var clusters [][]rune

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		// Check if r is a combining mark (Basic Arabic Range)
		// 064B-065F: Tashkeel
		// 0670: Superscript Alef
		// 0610-061A: Honorifics etc
		// 06D6-06DC: Quranic marks
		// 06DF-06E4: More marks
		// 06E7-06E8: More
		// 06EA-06ED: More
		if isCombiningMark(r) && len(clusters) > 0 {
			clusters[len(clusters)-1] = append(clusters[len(clusters)-1], r)
		} else {
			clusters = append(clusters, []rune{r})
		}
	}

	// Reverse clusters
	for i, j := 0, len(clusters)-1; i < j; i, j = i+1, j-1 {
		clusters[i], clusters[j] = clusters[j], clusters[i]
	}

	// Flatten
	var res []rune
	for _, cluster := range clusters {
		res = append(res, cluster...)
	}
	return string(res)
}

func isCombiningMark(r rune) bool {
	return (r >= 0x064B && r <= 0x065F) || // Fathatan, Dammatan, Kasratan, Fatha, Damma, Kasra, Shadda, Sukun
		r == 0x0670 || // Superscript Alef
		(r >= 0x0610 && r <= 0x061A) ||
		(r >= 0x06D6 && r <= 0x06DC) ||
		(r >= 0x06DF && r <= 0x06E4) ||
		(r >= 0x06E7 && r <= 0x06E8) ||
		(r >= 0x06EA && r <= 0x06ED)
}
