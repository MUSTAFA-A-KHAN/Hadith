package image

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

//go:embed template.html
var templateFS embed.FS

type Generator struct {
	fontDir  string
	bgDir    string
	bgImages []string // file paths for custom backgrounds
	htmlTmpl *template.Template

	// Cached font data
	englishFontData string
	amiriFontData   string
	classicFontData string
}

func NewGenerator(fontDir, bgDir string) *Generator {
	var bgFiles []string
	if bgDir != "" {
		jpegFiles, _ := filepath.Glob(filepath.Join(bgDir, "*.jpeg"))
		jpgFiles, _ := filepath.Glob(filepath.Join(bgDir, "*.jpg"))
		bgFiles = append(jpegFiles, jpgFiles...)
	}

	tmpl, err := template.ParseFS(templateFS, "template.html")
	if err != nil {
		panic(fmt.Errorf("failed to load embedded HTML template: %w", err))
	}

	g := &Generator{
		fontDir:  fontDir,
		bgDir:    bgDir,
		bgImages: bgFiles,
		htmlTmpl: tmpl,
	}

	// Pre-load and cache fonts to avoid reading and base64 encoding from disk on every generation request.
	if data, err := g.loadFontData("Caveat-Regular.ttf"); err == nil {
		g.englishFontData = data
	} else {
		fmt.Printf("Warning: failed to load english font: %v\n", err)
	}

	if data, err := g.loadFontData("Amiri-Regular.ttf"); err == nil {
		g.amiriFontData = data
	} else {
		fmt.Printf("Warning: failed to load amiri font: %v\n", err)
	}

	if data, err := g.loadFontData("ScheherazadeNew-Regular.ttf"); err == nil {
		g.classicFontData = data
	} else {
		fmt.Printf("Warning: failed to load classic arabic font: %v\n", err)
	}

	return g
}

func (g *Generator) loadFontData(fontName string) (string, error) {
	path := filepath.Join(g.fontDir, fontName)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return "data:font/truetype;charset=utf-8;base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (g *Generator) getRandomCustomBackgroundData() (string, error) {
	if len(g.bgImages) == 0 {
		return "", fmt.Errorf("no background images found")
	}

	path := g.bgImages[rand.Intn(len(g.bgImages))]
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Default to jpeg for simple data URI
	mimeType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(path), ".png") {
		mimeType = "image/png"
	}

	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
}

type templateData struct {
	Title           string
	Narrator        template.HTML
	ArabicText      string
	EnglishText     template.HTML
	Reference       string
	UseCustomBg     bool
	EnglishFontData template.URL
	ArabicFontData  template.URL
	AmiriFontData   template.URL
	BgImageData     template.URL
}

func processTextWithSawSymbol(text string) template.HTML {
	// Escape HTML to prevent injection but keep our span safe
	escaped := template.HTMLEscapeString(text)
	escaped = strings.ReplaceAll(escaped, "(saw)", "ﷺ")
	escaped = strings.ReplaceAll(escaped, "(pbuh)", "ﷺ")
	escaped = strings.ReplaceAll(escaped, "ﷺ", `<span class="saw-symbol">ﷺ</span>`)
	return template.HTML(escaped)
}

func (g *Generator) GenerateHadithImage(title, narrator, arabicText, englishText, reference string, useCustomBg bool, useClassicArabic bool) ([]byte, error) {
	// 1. Prepare Template Data
	var err error
	var bgData string
	if useCustomBg && g.bgDir != "" {
		bgData, err = g.getRandomCustomBackgroundData()
		if err != nil {
			useCustomBg = false // Fallback
		}
	}

	arabicFontData := g.amiriFontData
	if useClassicArabic {
		arabicFontData = g.classicFontData
	}

	// Prepare attribution
	if narrator == "" {
		narrator = "The Prophet Muhammad ﷺ said:"
	} else if !strings.HasSuffix(narrator, ":") && !strings.HasSuffix(narrator, ".") {
		narrator += ":"
	}

	data := templateData{
		Title:           strings.ToUpper(title),
		Narrator:        processTextWithSawSymbol(narrator),
		ArabicText:      arabicText, // pure HTML handles RTL natively, no garabic shaping needed!
		EnglishText:     processTextWithSawSymbol(englishText),
		Reference:       reference,
		UseCustomBg:     useCustomBg,
		EnglishFontData: template.URL(g.englishFontData),
		ArabicFontData:  template.URL(arabicFontData),
		AmiriFontData:   template.URL(g.amiriFontData),
		BgImageData:     template.URL(bgData),
	}

	// 2. Render HTML
	var buf bytes.Buffer
	if err := g.htmlTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute html template: %w", err)
	}

	htmlContent := buf.String()

	// 3. Render HTML to Image via ChromeDP
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Add timeout
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var imageBuf []byte

	// The width is fixed at 1080px. We need Chrome to capture the full scrolling height.
	err = chromedp.Run(ctx,
		// Load HTML directly
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frameTree.Frame.ID, htmlContent).Do(ctx)
		}),

		// Set a base viewport to trigger responsive rendering, but height will dynamically grow
		emulation.SetDeviceMetricsOverride(1080, 1080, 1, false),

		// Wait robustly for fonts to load and rendering to settle
		chromedp.EvaluateAsDevTools(`new Promise(resolve => document.fonts.ready.then(resolve))`, nil),

		// Capture full page screenshot
		chromedp.FullScreenshot(&imageBuf, 100),
	)

	if err != nil {
		return nil, fmt.Errorf("chromedp failed to render image: %w", err)
	}

	return imageBuf, nil
}
