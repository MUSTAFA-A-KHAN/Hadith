package models

import (
	"time"
)

// Collection represents a hadith collection
type Collection struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Hadiths     int    `json:"hadiths"`
	Books       int    `json:"books"`
	Description string `json:"description"`
	Grade       string `json:"grade"`
}

// Book represents a book/chapter in a collection
type Book struct {
	BookNumber   int    `json:"bookNumber"`
	Title        string `json:"title"`
	EnglishTitle string `json:"englishTitle"`
	ArabicTitle  string `json:"arabicTitle"`
	HadithCount  int    `json:"hadithCount"`
	ChapterID    int    `json:"chapterId"`
}

// Hadith represents a single hadith
type Hadith struct {
	HadithNumber   int    `json:"hadithNumber"`
	Grade          string `json:"grade"`
	Arabic         string `json:"arabic"`
	English        string `json:"english"`
	Narrator       string `json:"narrator"`
	ChapterID      int    `json:"chapterId"`
	BookID         int    `json:"bookId"`
	CollectionName string `json:"collectionName,omitempty"` // Added to track which collection a search result came from
}

// HadithResponse represents the response from getting hadiths
type HadithResponse struct {
	Hadiths    []Hadith `json:"hadiths"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	TotalPages int      `json:"totalPages"`
}

// SearchResult represents search results
type SearchResult struct {
	Hadiths    []Hadith `json:"hadiths"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	TotalPages int      `json:"totalPages"`
}

// RandomHadithResult represents a random hadith result
type RandomHadithResult struct {
	Hadith     *Hadith     `json:"hadith"`
	Collection *Collection `json:"collection"`
	Book       *Book       `json:"book"`
}

// CollectionData holds all collection data
type CollectionData struct {
	Collections []Collection
	Books       map[string][]Book
	Hadiths     map[string][]Hadith
}

// GetCollections returns all available collections
func (d *CollectionData) GetCollections() []Collection {
	return d.Collections
}

// GetCollection returns a collection by name
func (d *CollectionData) GetCollection(name string) *Collection {
	for i := range d.Collections {
		if d.Collections[i].Name == name {
			return &d.Collections[i]
		}
	}
	return nil
}

// GetBooks returns all books in a collection
func (d *CollectionData) GetBooks(collection string) []Book {
	if books, ok := d.Books[collection]; ok {
		return books
	}
	return nil
}

// GetBook returns a specific book in a collection
func (d *CollectionData) GetBook(collection string, bookNumber int) *Book {
	if books, ok := d.Books[collection]; ok {
		for i := range books {
			if books[i].BookNumber == bookNumber {
				return &books[i]
			}
		}
	}
	return nil
}

// GetHadiths returns hadiths from a collection and book with pagination
func (d *CollectionData) GetHadiths(collection string, bookNumber int, page int, limit int) HadithResponse {
	hadiths, ok := d.Hadiths[collection]
	if !ok {
		return HadithResponse{
			Hadiths:    []Hadith{},
			Total:      0,
			Page:       page,
			TotalPages: 0,
		}
	}

	// Filter by book if specified
	var filtered []Hadith
	if bookNumber > 0 {
		for _, h := range hadiths {
			if h.ChapterID == bookNumber {
				filtered = append(filtered, h)
			}
		}
	} else {
		filtered = hadiths
	}

	total := len(filtered)
	totalPages := (total + limit - 1) / limit

	if page < 1 {
		page = 1
	}

	start := (page - 1) * limit
	if start >= total {
		return HadithResponse{
			Hadiths:    []Hadith{},
			Total:      total,
			Page:       page,
			TotalPages: totalPages,
		}
	}

	end := start + limit
	if end > total {
		end = total
	}

	return HadithResponse{
		Hadiths:    filtered[start:end],
		Total:      total,
		Page:       page,
		TotalPages: totalPages,
	}
}

// SearchHadiths searches hadiths by keyword
func (d *CollectionData) SearchHadiths(query string, page int, limit int) SearchResult {
	query = toLower(query)
	var results []Hadith

	for collectionName, hadiths := range d.Hadiths {
		for _, h := range hadiths {
			if contains(toLower(h.English), query) ||
			   contains(h.Arabic, query) ||
			   contains(toLower(h.Narrator), query) {
				h.CollectionName = collectionName // Save the collection name for the search result
				results = append(results, h)
			}
		}
	}

	total := len(results)
	totalPages := (total + limit - 1) / limit

	if page < 1 {
		page = 1
	}

	start := (page - 1) * limit
	if start >= total {
		return SearchResult{
			Hadiths:    []Hadith{},
			Total:      total,
			Page:       page,
			TotalPages: totalPages,
		}
	}

	end := start + limit
	if end > total {
		end = total
	}

	return SearchResult{
		Hadiths:    results[start:end],
		Total:      total,
		Page:       page,
		TotalPages: totalPages,
	}
}

// GetRandomHadith returns a random hadith
func (d *CollectionData) GetRandomHadith() RandomHadithResult {
	// Get all collections
	collections := d.Collections
	if len(collections) == 0 {
		return RandomHadithResult{}
	}

	// Pick a random collection (prefer major ones)
	majorCollections := []string{"bukhari", "muslim", "abudawud", "tirmidhi", "nasai", "ibnmajah"}
	var selectedCollections []Collection
	for _, name := range majorCollections {
		if c := d.GetCollection(name); c != nil {
			selectedCollections = append(selectedCollections, *c)
		}
	}
	if len(selectedCollections) == 0 {
		selectedCollections = collections
	}

	collection := selectedCollections[randInt(len(selectedCollections))]

	// Get books for this collection
	books := d.GetBooks(collection.Name)
	if len(books) == 0 {
		return RandomHadithResult{}
	}

	// Pick a random book
	book := books[randInt(len(books))]

	// Get hadiths for this collection
	hadiths := d.Hadiths[collection.Name]
	if len(hadiths) == 0 {
		return RandomHadithResult{}
	}

	// Pick a random hadith
	hadith := hadiths[randInt(len(hadiths))]

	return RandomHadithResult{
		Hadith:     &hadith,
		Collection: &collection,
		Book:       &book,
	}
}

// Helper functions
func toLower(s string) string {
	result := make([]rune, len(s))
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + 32
		} else {
			result[i] = r
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Simple random int (avoid import for simplicity)
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	// Use current time nanoseconds for simple randomness
	return int(time.Now().UnixNano()) % n
}

