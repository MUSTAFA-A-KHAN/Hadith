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

	// Common max width
	maxWidth := float64(W) - 160

	// Title Height
	if err := measureDC.LoadFontFace(englishFontPath, 110); err != nil {
		return nil, fmt.Errorf("failed to load title font: %w", err)
	}
	titleLines := measureDC.WordWrap(strings.ToUpper(title), maxWidth)
	titleLineHeight := measureDC.FontHeight() * 1.1
	titleHeight := float64(len(titleLines)) * titleLineHeight

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

	attributionLines := measureDC.WordWrap(displayText, maxWidth)
	attributionHeight := float64(len(attributionLines)) * measureDC.FontHeight() * 1.2

	// Arabic Height
	if err := measureDC.LoadFontFace(arabicFontPath, 70); err != nil {
		return nil, fmt.Errorf("failed to load arabic font: %w", err)
	}

	// Sanitize Arabic text
	safeArabicText := strings.TrimSpace(arabicText)
	if safeArabicText == "" {
		safeArabicText = " "
	}

	// Wrap Logical Text first to preserve sentence order (Top-to-Bottom)
	logicalArabicLines := measureDC.WordWrap(safeArabicText, maxWidth)

	// Shape each line (Logical -> Visual)
	var shapedArabicLines []string
	for _, line := range logicalArabicLines {
		var shaped string
		func() {
			defer func() {
				if r := recover(); r != nil {
					shaped = line // Fallback to logical text (better than crash)
				}
			}()
			// garabic.Shape returns Visual Order (Right-to-Left characters reversed for LTR display)
			shaped = garabic.Shape(line)
		}()
		shapedArabicLines = append(shapedArabicLines, shaped)
	}

	arabicLineHeight := measureDC.FontHeight() * 1.5
	arabicTotalHeight := float64(len(shapedArabicLines)) * arabicLineHeight

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
	gap1 := 80.0 // Title to attribution
	gap2 := 100.0 // Attribution to Arabic
	gap3 := 80.0 // Arabic to English
	gap4 := 100.0 // English to Reference
	paddingBottom := 100.0

	// Flow calculation
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
	currentTitleY := titleY - (titleHeight / 2) + (titleLineHeight / 2)
	for _, line := range titleLines {
		dc.DrawStringAnchored(line, float64(W)/2, currentTitleY, 0.5, 0.5)
		currentTitleY += titleLineHeight
	}

	// Draw Attribution
	dc.SetHexColor("#1a1a1a")
	if strings.Contains(displayText, "ﷺ") {
		// Centered single line approach for mixed font (simplified)
		// We use the first line of attributionLines if wrapping happened, but mixed font wrapping is complex.
		// Fallback: Just draw the first line or try to draw full string centered if short.
		// Given constraints, we'll iterate wrapped lines but only support symbol in them if they fit logic.
		// Simplification: Iterate attributionLines (which are English font wrapped).
		// If a line has the placeholder, we split and draw.

		currentAttrY := attributionY - (attributionHeight/2) + (measureDC.FontHeight()*1.2/2)
		for _, line := range attributionLines {
			if strings.Contains(line, "ﷺ") {
				parts := strings.Split(line, "ﷺ")
				totalW := 0.0

				// Measure total width first
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
					dc.DrawStringAnchored(part, curX, currentAttrY, 0, 0.5)
					w, _ := dc.MeasureString(part)
					curX += w
					if i < len(parts)-1 {
						dc.LoadFontFace(arabicFontPath, 50)
						dc.DrawStringAnchored("ﷺ", curX, currentAttrY, 0, 0.5)
						w, _ = dc.MeasureString("ﷺ")
						curX += w
					}
				}
			} else {
				dc.LoadFontFace(englishFontPath, 50)
				dc.DrawStringAnchored(line, float64(W)/2, currentAttrY, 0.5, 0.5)
			}
			currentAttrY += measureDC.FontHeight() * 1.2
		}
	} else {
		dc.LoadFontFace(englishFontPath, 50)
		for i, line := range attributionLines {
			offsetY := float64(i)*measureDC.FontHeight()*1.2 - (attributionHeight/2) + (measureDC.FontHeight()*1.2/2)
			dc.DrawStringAnchored(line, float64(W)/2, attributionY+offsetY, 0.5, 0.5)
		}
	}

	// Draw Arabic
	dc.SetHexColor("#000000")
	dc.LoadFontFace(arabicFontPath, 70)
	for i, line := range shapedArabicLines {
		// line is already shaped and in Visual Order.
		// gg draws LTR. Visual Order is designed for LTR.
		// So we just draw it.

		offsetY := float64(i)*arabicLineHeight - (arabicTotalHeight/2) + (arabicLineHeight/2)
		dc.DrawStringAnchored(line, float64(W)/2, arabicStartY+offsetY, 0.5, 0.5)
	}

	// Draw English
	dc.SetHexColor("#1a1a1a")
	dc.LoadFontFace(englishFontPath, 60)
	for i, line := range englishLines {
		offsetY := float64(i)*englishLineHeight - (englishTotalHeight/2) + (englishLineHeight/2)
		lineY := englishStartY + offsetY

		if strings.Contains(line, "ﷺ") {
			parts := strings.Split(line, "ﷺ")
			totalW := 0.0

			// Measure total width first
			for i, part := range parts {
				dc.LoadFontFace(englishFontPath, 60)
				w, _ := dc.MeasureString(part)
				totalW += w
				if i < len(parts)-1 {
					dc.LoadFontFace(arabicFontPath, 60)
					w, _ = dc.MeasureString("ﷺ")
					totalW += w
				}
			}

			startX := (float64(W) - totalW) / 2
			curX := startX

			for i, part := range parts {
				dc.LoadFontFace(englishFontPath, 60)
				dc.DrawStringAnchored(part, curX, lineY, 0, 0.5)
				w, _ := dc.MeasureString(part)
				curX += w
				if i < len(parts)-1 {
					dc.LoadFontFace(arabicFontPath, 60)
					dc.DrawStringAnchored("ﷺ", curX, lineY, 0, 0.5)
					w, _ = dc.MeasureString("ﷺ")
					curX += w
				}
			}
		} else {
			dc.LoadFontFace(englishFontPath, 60)
			dc.DrawStringAnchored(line, float64(W)/2, lineY, 0.5, 0.5)
		}
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
