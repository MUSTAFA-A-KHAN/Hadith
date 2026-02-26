package bot

import (
	"hadith-bot/internal/models"
	"strings"
	"testing"
)

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "Hello"},
		{"<b>Bold</b>", "&lt;b&gt;Bold&lt;/b&gt;"},
		{"&", "&amp;"},
		{`"`, "&#34;"},
		{"'", "&#39;"},
	}

	for _, tt := range tests {
		got := escapeHTML(tt.input)
		if got != tt.expected {
			t.Errorf("escapeHTML(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatHadithDisplay(t *testing.T) {
	h := &Handler{} // Nil dependencies are fine for this method as it doesn't use them

	hadith := &models.Hadith{
		HadithNumber: 1,
		Grade:        "Sahih",
		Arabic:       "Arabic Text <>&",
		English:      "English Text <>&",
		Narrator:     "Narrator <>&",
		ChapterID:    1,
	}
	col := &models.Collection{
		Title: "Collection Title <>&",
	}
	book := &models.Book{
		BookNumber: 1,
	}

	got := h.formatHadithDisplay(hadith, col, book)

	// Check for HTML escaping
	if !strings.Contains(got, "Arabic Text &lt;&gt;&amp;") {
		t.Errorf("Result should contain escaped Arabic text, got: %s", got)
	}
	if !strings.Contains(got, "English Text &lt;&gt;&amp;") {
		t.Errorf("Result should contain escaped English text, got: %s", got)
	}
	if !strings.Contains(got, "Narrator &lt;&gt;&amp;") {
		t.Errorf("Result should contain escaped Narrator, got: %s", got)
	}
	if !strings.Contains(got, "Collection Title &lt;&gt;&amp;") {
		t.Errorf("Result should contain escaped Collection Title, got: %s", got)
	}

	// Check for HTML tags
	if !strings.Contains(got, "<b>Hadith</b>") {
		t.Errorf("Result should contain bold Hadith title")
	}
	if !strings.Contains(got, "<b>Narrator:</b>") {
		t.Errorf("Result should contain bold Narrator label")
	}
}
