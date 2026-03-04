package image

import (
	"bytes"
	"fmt"
	"image"
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
	bgDir   string
	bgImages []image.Image
}

func NewGenerator(fontDir, bgDir string) *Generator {
	var images []image.Image
	if bgDir != "" {
		jpegFiles, _ := filepath.Glob(filepath.Join(bgDir, "*.jpeg"))
		jpgFiles, _ := filepath.Glob(filepath.Join(bgDir, "*.jpg"))
		files := append(jpegFiles, jpgFiles...)
		for _, file := range files {
			if im, err := gg.LoadImage(file); err == nil {
				images = append(images, im)
			}
		}
	}

	return &Generator{
		fontDir:  fontDir,
		bgDir:    bgDir,
		bgImages: images,
	}
}

func (g *Generator) GenerateHadithImage(title, narrator, arabicText, englishText, reference string, useCustomBg bool) ([]byte, error) {
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

	textColorMain := "#1a1a1a"
	textColorRef := "#4a4a4a"
	titleColor := "#558B2F"

	if useCustomBg && g.bgDir != "" {
		im, err := g.getRandomCustomBackground()
		if err == nil {
			g.drawCustomBackground(dc, im, W, totalH)
			// For custom backgrounds, we overlay a dimming layer
			dc.SetRGBA(0, 0, 0, 0.6)
			dc.DrawRectangle(0, 0, float64(W), float64(totalH))
			dc.Fill()

			// Use white/light colors for text to be readable on dark overlay
			textColorMain = "#FFFFFF"
			textColorRef = "#DDDDDD"
			titleColor = "#FFFFFF"
		} else {
			g.drawBackground(dc)
		}
	} else {
		g.drawBackground(dc)
	}

	// Draw Title
	dc.SetHexColor(titleColor)
	dc.LoadFontFace(englishFontPath, 110)
	currentTitleY := titleY - (titleHeight / 2) + (titleLineHeight / 2)
	for _, line := range titleLines {
		dc.DrawStringAnchored(line, float64(W)/2, currentTitleY, 0.5, 0.5)
		currentTitleY += titleLineHeight
	}

	// Draw Attribution
	dc.SetHexColor(textColorMain)
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
	dc.SetHexColor(textColorMain)
	dc.LoadFontFace(arabicFontPath, 70)
	for i, line := range shapedArabicLines {
		// line is already shaped and in Visual Order.
		// gg draws LTR. Visual Order is designed for LTR.
		// So we just draw it.

		offsetY := float64(i)*arabicLineHeight - (arabicTotalHeight/2) + (arabicLineHeight/2)
		dc.DrawStringAnchored(line, float64(W)/2, arabicStartY+offsetY, 0.5, 0.5)
	}

	// Draw English
	dc.SetHexColor(textColorMain)
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
	dc.SetHexColor(textColorRef)
	dc.LoadFontFace(englishFontPath, 40)
	dc.DrawStringAnchored(reference, float64(W)/2, refY, 0.5, 0.5)

	// Draw Bismillah Header (Decorative)
	dc.LoadFontFace(arabicFontPath, 40)
	if titleColor == "#FFFFFF" {
		dc.SetHexColor("#FFFFFF")
	} else {
		dc.SetHexColor("#556B2F") // Olive
	}
	dc.DrawStringAnchored(garabic.Shape("بسم الله الرحمن الرحيم"), float64(W)/2, 65, 0.5, 0.5)

	var buf bytes.Buffer
	if err := dc.EncodePNG(&buf); err != nil {
		return nil, fmt.Errorf("failed to encode png: %w", err)
	}

	return buf.Bytes(), nil
}

func (g *Generator) drawBackground(dc *gg.Context) {
	width := float64(dc.Width())
	height := float64(dc.Height())

	// 1. Soft Warm Beige Background
	dc.SetHexColor("#FDFCF5") // Very light cream/warm paper
	dc.Clear()

	// 2. Subtle Texture (Noise/Speckles)
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 5000; i++ {
		x := rnd.Float64() * width
		y := rnd.Float64() * height
		r := 0.5 + rnd.Float64()*1.5
		alpha := 10 + rnd.Intn(20)
		dc.SetRGBA255(210, 205, 190, alpha) // darker beige speckles
		dc.DrawCircle(x, y, r)
		dc.Fill()
	}

	// 3. Islamic Geometric Pattern (Simplified Star/Rosette Motif) - Faint overlay
	dc.SetRGBA255(180, 190, 160, 15) // Sage green tint, very transparent
	patternSize := 150.0
	for x := 0.0; x < width; x += patternSize {
		for y := 0.0; y < height; y += patternSize {
			drawStarMotif(dc, x+patternSize/2, y+patternSize/2, patternSize*0.4)
		}
	}

	// 4. Elegant Border
	margin := 30.0
	dc.SetLineWidth(3)
	dc.SetHexColor("#8FBC8F") // Dark Sea Green
	dc.DrawRectangle(margin, margin, width-2*margin, height-2*margin)
	dc.Stroke()

	// Inner thin gold border
	margin2 := 38.0
	dc.SetLineWidth(1)
	dc.SetHexColor("#D4AF37") // Gold
	dc.DrawRectangle(margin2, margin2, width-2*margin2, height-2*margin2)
	dc.Stroke()

	// Corner Accents (Floral/Geometric)
	drawCorner(dc, margin, margin, 1)            // Top-Left
	drawCorner(dc, width-margin, margin, 2)      // Top-Right
	drawCorner(dc, width-margin, height-margin, 3) // Bottom-Right
	drawCorner(dc, margin, height-margin, 4)     // Bottom-Left
}

func drawStarMotif(dc *gg.Context, cx, cy, r float64) {
	dc.Push()
	dc.Translate(cx, cy)
	for i := 0; i < 8; i++ {
		dc.Rotate(gg.Radians(45))
		dc.DrawEllipse(0, r/2, r/6, r/2)
	}
	dc.Fill()
	dc.Pop()
}

func drawCorner(dc *gg.Context, x, y float64, corner int) {
	size := 80.0
	dc.Push()
	dc.Translate(x, y)

	// Rotate based on corner to face inward
	switch corner {
	case 1: // TL
		// No rotation
	case 2: // TR
		dc.Rotate(gg.Radians(90))
	case 3: // BR
		dc.Rotate(gg.Radians(180))
	case 4: // BL
		dc.Rotate(gg.Radians(270))
	}

	// Draw decorative vine/leaf
	dc.SetHexColor("#556B2F") // Dark Olive Green
	dc.MoveTo(0, 0)
	dc.QuadraticTo(size/2, 0, size, size)
	dc.Stroke()

	// Leaf
	dc.SetRGBA255(107, 142, 35, 100) // Olive Drab
	dc.DrawCircle(size/3, size/3, 5)
	dc.Fill()
	dc.DrawCircle(size/1.5, size/1.5, 3)
	dc.Fill()

	dc.Pop()
}

func (g *Generator) getFontPath(fontName string) string {
	return filepath.Join(g.fontDir, fontName)
}

func (g *Generator) getRandomCustomBackground() (image.Image, error) {
	if len(g.bgImages) == 0 {
		return nil, fmt.Errorf("no background images found")
	}

	return g.bgImages[rand.Intn(len(g.bgImages))], nil
}

func (g *Generator) drawCustomBackground(dc *gg.Context, im image.Image, W, H int) {
	iw := im.Bounds().Dx()
	ih := im.Bounds().Dy()

	// Scale and center crop to fill the canvas
	scale := math.Max(float64(W)/float64(iw), float64(H)/float64(ih))

	newW := float64(iw) * scale
	newH := float64(ih) * scale

	x := (float64(W) - newW) / 2
	y := (float64(H) - newH) / 2

	dc.Push()
	dc.Translate(x, y)
	dc.Scale(scale, scale)
	dc.DrawImage(im, 0, 0)
	dc.Pop()
}
