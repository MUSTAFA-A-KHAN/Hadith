package image

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"strings"
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

func (g *Generator) GenerateHadithImage(title, narrator, arabicText, englishText, reference string) ([]byte, error) {
	const W = 1080
	// 1. Measure text to determine dynamic height
	measureDC := gg.NewContext(W, 100)

	arabicFontPath := g.getFontPath("Amiri-Regular.ttf")
	englishFontPath := g.getFontPath("Caveat-Regular.ttf")

	// --- Calculations ---

	// Title Height
	if err := measureDC.LoadFontFace(englishFontPath, 110); err != nil {
		return nil, fmt.Errorf("failed to load title font: %w", err)
	}
	titleHeight := measureDC.FontHeight()

	// Attribution Height
	displayText := narrator
	if displayText == "" {
		displayText = "The Prophet Muhammad ﷺ said:"
	} else {
		if !strings.HasSuffix(displayText, ":") && !strings.HasSuffix(displayText, ".") {
			displayText += ":"
		}
	}
	displayText = strings.ReplaceAll(displayText, "(saw)", "ﷺ")
	displayText = strings.ReplaceAll(displayText, "(pbuh)", "ﷺ")

	if err := measureDC.LoadFontFace(englishFontPath, 50); err != nil {
		return nil, fmt.Errorf("failed to load attribution font: %w", err)
	}
	// Simplified measurement: max font height (assuming single line for now, or minimal wrapping)
	// If the narrator name is extremely long, we might need wrapping, but let's assume single line or minimal.
	// Actually, let's wrap just in case.
	maxWidth := float64(W) - 160
	attributionLines := measureDC.WordWrap(displayText, maxWidth)
	attributionHeight := float64(len(attributionLines)) * measureDC.FontHeight() * 1.2

	// Arabic Height
	if err := measureDC.LoadFontFace(arabicFontPath, 70); err != nil {
		return nil, fmt.Errorf("failed to load arabic font: %w", err)
	}
	shapedArabic := garabic.Shape(arabicText)
	arabicLines := measureDC.WordWrap(shapedArabic, maxWidth)
	arabicLineHeight := measureDC.FontHeight() * 1.5
	arabicTotalHeight := float64(len(arabicLines)) * arabicLineHeight

	// English Height
	if err := measureDC.LoadFontFace(englishFontPath, 60); err != nil {
		return nil, fmt.Errorf("failed to load english font: %w", err)
	}
	englishLines := measureDC.WordWrap(englishText, maxWidth)
	englishLineHeight := measureDC.FontHeight() * 1.2
	englishTotalHeight := float64(len(englishLines)) * englishLineHeight

	// Reference Height
	if err := measureDC.LoadFontFace(englishFontPath, 40); err != nil {
		return nil, fmt.Errorf("failed to load ref font: %w", err)
	}
	refHeight := measureDC.FontHeight()

	// Padding & Spacing
	// paddingTop := 150.0 // Unused
	gap1 := 80.0 // Title to attribution
	gap2 := 100.0 // Attribution to Arabic
	gap3 := 80.0 // Arabic to English
	gap4 := 100.0 // English to Reference
	paddingBottom := 100.0

	// Total required height calculation
	// We anchor Title at Y=150. So top used space is roughly 150 + titleHeight/2.
	// Let's flow from top instead of anchoring.

	currentY := 100.0 // Top margin

	titleY := currentY + titleHeight/2
	currentY += titleHeight + gap1

	attributionY := currentY + attributionHeight/2
	currentY += attributionHeight + gap2

	arabicStartY := currentY + arabicTotalHeight/2
	currentY += arabicTotalHeight + gap3

	englishStartY := currentY + englishTotalHeight/2
	currentY += englishTotalHeight + gap4

	refY := currentY + refHeight/2
	currentY += refHeight + paddingBottom

	totalH := int(math.Max(1080, currentY))

	// --- 2. Drawing ---
	dc := gg.NewContext(W, totalH)
	g.drawBackground(dc)

	// Draw Title
	dc.SetHexColor("#558B2F")
	dc.LoadFontFace(englishFontPath, 110)
	dc.DrawStringAnchored(strings.ToUpper(title), float64(W)/2, titleY, 0.5, 0.5)

	// Draw Attribution
	dc.SetHexColor("#1a1a1a")
	// For simplicity in dynamic layout with mixed fonts, we center the whole block at attributionY.
	// If it wraps, DrawStringAnchored handles it for single font, but we have mixed font complexity.
	// Let's simplify mixed font handling: If it contains symbol, render segments.
	// Limitation: Mixed font + Wrapping is hard.
	// For now, we assume Attribution fits in one or two lines and center it.

	if strings.Contains(displayText, "ﷺ") {
		// Centered single line approach for mixed font
		parts := strings.Split(displayText, "ﷺ")
		totalW := 0.0
		for i, part := range parts {
			dc.LoadFontFace(englishFontPath, 50)
			w, _ := dc.MeasureString(part)
			totalW += w
			if i < len(parts)-1 {
				dc.LoadFontFace(arabicFontPath, 50)
				w, _ = dc.MeasureString("ﷺ")
				totalW += w
			}
		}
		startX := (float64(W) - totalW) / 2
		curX := startX
		for i, part := range parts {
			dc.LoadFontFace(englishFontPath, 50)
			dc.DrawStringAnchored(part, curX, attributionY, 0, 0.5)
			w, _ := dc.MeasureString(part)
			curX += w
			if i < len(parts)-1 {
				dc.LoadFontFace(arabicFontPath, 50)
				dc.DrawStringAnchored("ﷺ", curX, attributionY, 0, 0.5)
				w, _ = dc.MeasureString("ﷺ")
				curX += w
			}
		}
	} else {
		dc.LoadFontFace(englishFontPath, 50)
		// Handle wrapping if needed
		for i, line := range attributionLines {
			offsetY := float64(i)*measureDC.FontHeight()*1.2 - (attributionHeight/2) + (measureDC.FontHeight()*1.2/2)
			dc.DrawStringAnchored(line, float64(W)/2, attributionY+offsetY, 0.5, 0.5)
		}
	}

	// Draw Arabic
	dc.SetHexColor("#000000")
	dc.LoadFontFace(arabicFontPath, 70)
	for i, line := range arabicLines {
		reversedLine := g.reversePreservingCombiningMarks(line)
		offsetY := float64(i)*arabicLineHeight - (arabicTotalHeight/2) + (arabicLineHeight/2)
		dc.DrawStringAnchored(reversedLine, float64(W)/2, arabicStartY+offsetY, 0.5, 0.5)
	}

	// Draw English
	dc.SetHexColor("#1a1a1a")
	dc.LoadFontFace(englishFontPath, 60)
	for i, line := range englishLines {
		offsetY := float64(i)*englishLineHeight - (englishTotalHeight/2) + (englishLineHeight/2)
		dc.DrawStringAnchored(line, float64(W)/2, englishStartY+offsetY, 0.5, 0.5)
	}

	// Draw Reference
	dc.SetHexColor("#4a4a4a")
	dc.LoadFontFace(englishFontPath, 40)
	dc.DrawStringAnchored(reference, float64(W)/2, refY, 0.5, 0.5)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("failed to encode png: %w", err)
	}

	return buf.Bytes(), nil
}

func (g *Generator) drawBackground(dc *gg.Context) {
	// Light blue/white tint
	dc.SetHexColor("#F0F8FF")
	dc.Clear()

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	width := dc.Width()
	height := dc.Height()

	// Faint blobs
	// Scale number of blobs by height ratio
	numBlobs := int(5 * (float64(height) / 1080.0))
	if numBlobs < 5 { numBlobs = 5 }

	for i := 0; i < numBlobs; i++ {
		x := rnd.Float64() * float64(width)
		y := rnd.Float64() * float64(height)
		r := 100 + rnd.Float64()*200

		rCol := 200 + rnd.Intn(55)
		gCol := 200 + rnd.Intn(55)
		bCol := 200 + rnd.Intn(55)

		dc.SetRGBA255(rCol, gCol, bCol, 20)
		dc.DrawCircle(x, y, r)
		dc.Fill()
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
		if isCombiningMark(r) && len(clusters) > 0 {
			clusters[len(clusters)-1] = append(clusters[len(clusters)-1], r)
		} else {
			clusters = append(clusters, []rune{r})
		}
	}

	for i, j := 0, len(clusters)-1; i < j; i, j = i+1, j-1 {
		clusters[i], clusters[j] = clusters[j], clusters[i]
	}

	var res []rune
	for _, cluster := range clusters {
		res = append(res, cluster...)
	}
	return string(res)
}

func isCombiningMark(r rune) bool {
	return (r >= 0x064B && r <= 0x065F) ||
		r == 0x0670 ||
		(r >= 0x0610 && r <= 0x061A) ||
		(r >= 0x06D6 && r <= 0x06DC) ||
		(r >= 0x06DF && r <= 0x06E4) ||
		(r >= 0x06E7 && r <= 0x06E8) ||
		(r >= 0x06EA && r <= 0x06ED)
}
