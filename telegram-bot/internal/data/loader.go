package data

import (
	"encoding/json"
	"hadith-bot/internal/models"
	"os"
	"path/filepath"
)

// LoadHadithData loads hadith data from JSON files
func LoadHadithData(dataDir string) (*models.CollectionData, error) {
	data := &models.CollectionData{
		Books:   make(map[string][]models.Book),
		Hadiths: make(map[string][]models.Hadith),
	}

	// Define major collections
	collections := []models.Collection{
		{
			Name:        "bukhari",
			Title:       "Sahih al-Bukhari",
			Author:      "Imam al-Bukhari",
			Hadiths:     0,
			Books:       0,
			Description: "The most authentic collection of hadith",
			Grade:       "Sahih",
		},
		{
			Name:        "muslim",
			Title:       "Sahih Muslim",
			Author:      "Imam Muslim",
			Hadiths:     0,
			Books:       0,
			Description: "The second most authentic collection",
			Grade:       "Sahih",
		},
		{
			Name:        "abudawud",
			Title:       "Sunan Abu Dawood",
			Author:      "Abu Dawood",
			Hadiths:     0,
			Books:       0,
			Description: "Collection of hadith focusing on jurisprudential matters",
			Grade:       "Sahih",
		},
		{
			Name:        "tirmidhi",
			Title:       "Jami' at-Tirmidhi",
			Author:      "Imam at-Tirmidhi",
			Hadiths:     0,
			Books:       0,
			Description: "Comprehensive collection of hadith",
			Grade:       "Sahih",
		},
		{
			Name:        "nasai",
			Title:       "Sunan an-Nasa'i",
			Author:      "Imam an-Nasa'i",
			Hadiths:     0,
			Books:       0,
			Description: "Collection of hadith on jurisprudence",
			Grade:       "Sahih",
		},
		{
			Name:        "ibnmajah",
			Title:       "Sunan Ibn Majah",
			Author:      "Ibn Majah",
			Hadiths:     0,
			Books:       0,
			Description: "Collection of hadith on jurisprudence",
			Grade:       "Sahih",
		},
		{
			Name:        "riyadussaliheen",
			Title:       "Riyad as-Salihin",
			Author:      "Imam an-Nawawi",
			Hadiths:     0,
			Books:       0,
			Description: "The Meadows of the Righteous",
			Grade:       "Hasan",
		},
	}

	data.Collections = collections

	// Load data from each collection
	collectionFiles := []string{
		"bukhari.json",
		"muslim.json",
		"abudawud.json",
		"tirmidhi.json",
		"nasai.json",
		"ibnmajah.json",
		"riyadussaliheen.json",
	}

	for _, file := range collectionFiles {
		path := filepath.Join(dataDir, file)
		if _, err := os.Stat(path); err != nil {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Parse JSON
		var rawData map[string]interface{}
		if err := json.Unmarshal(content, &rawData); err != nil {
			continue
		}

		// Extract collection name from filename
		collectionName := file[:len(file)-5]

		// Load chapters (books)
		if chapters, ok := rawData["chapters"].([]interface{}); ok {
			var books []models.Book
			for _, ch := range chapters {
				if chMap, ok := ch.(map[string]interface{}); ok {
					book := models.Book{
						BookNumber:   getInt(chMap["id"]),
						Title:        getString(chMap["english"]),
						EnglishTitle: getString(chMap["english"]),
						ArabicTitle:  getString(chMap["arabic"]),
					}
					books = append(books, book)
				}
			}
			data.Books[collectionName] = books

			// Update collection book count
			for i := range data.Collections {
				if data.Collections[i].Name == collectionName {
					data.Collections[i].Books = len(books)
					break
				}
			}
		}

		// Load hadiths
		if hadiths, ok := rawData["hadiths"].([]interface{}); ok {
			var parsedHadiths []models.Hadith
			for _, h := range hadiths {
				if hMap, ok := h.(map[string]interface{}); ok {
					grade := getString(hMap["grade"])
					if grade == "" {
						grade = "Sahih"
					}

					// Get english text
					englishText := ""
					if eng, ok := hMap["english"]; ok {
						if engStr, ok := eng.(string); ok {
							englishText = engStr
						} else if engMap, ok := eng.(map[string]interface{}); ok {
							englishText = getString(engMap["text"])
						}
					}

					// Get narrator
					narrator := ""
					if engMap, ok := hMap["english"].(map[string]interface{}); ok {
						narrator = getString(engMap["narrator"])
					}

					hadith := models.Hadith{
						HadithNumber: getInt(hMap["idInBook"]),
						Grade:        grade,
						Arabic:       getString(hMap["arabic"]),
						English:      englishText,
						Narrator:     narrator,
						ChapterID:    getInt(hMap["chapterId"]),
					}
					parsedHadiths = append(parsedHadiths, hadith)
				}
			}
			data.Hadiths[collectionName] = parsedHadiths

			// Update collection hadith count
			for i := range data.Collections {
				if data.Collections[i].Name == collectionName {
					data.Collections[i].Hadiths = len(parsedHadiths)
					break
				}
			}
		}
	}

	return data, nil
}

// Helper functions
func getString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func getInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch v := v.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if v == "" {
			return 0
		}
		return 0
	}
	return 0
}

// GetDefaultCollectionData returns embedded default data
func GetDefaultCollectionData() *models.CollectionData {
	return &models.CollectionData{
		Collections: []models.Collection{
			{
				Name:        "bukhari",
				Title:       "Sahih al-Bukhari",
				Author:      "Imam al-Bukhari",
				Hadiths:     7000,
				Books:       97,
				Description: "The most authentic collection of hadith",
				Grade:       "Sahih",
			},
			{
				Name:        "muslim",
				Title:       "Sahih Muslim",
				Author:      "Imam Muslim",
				Hadiths:     7000,
				Books:       56,
				Description: "The second most authentic collection",
				Grade:       "Sahih",
			},
			{
				Name:        "abudawud",
				Title:       "Sunan Abu Dawood",
				Author:      "Abu Dawood",
				Hadiths:     5000,
				Books:       80,
				Description: "Collection of hadith focusing on jurisprudential matters",
				Grade:       "Sahih",
			},
			{
				Name:        "tirmidhi",
				Title:       "Jami' at-Tirmidhi",
				Author:      "Imam at-Tirmidhi",
				Hadiths:     4000,
				Books:       50,
				Description: "Comprehensive collection of hadith",
				Grade:       "Sahih",
			},
			{
				Name:        "nasai",
				Title:       "Sunan an-Nasa'i",
				Author:      "Imam an-Nasa'i",
				Hadiths:     5700,
				Books:       52,
				Description: "Collection of hadith on jurisprudence",
				Grade:       "Sahih",
			},
			{
				Name:        "ibnmajah",
				Title:       "Sunan Ibn Majah",
				Author:      "Ibn Majah",
				Hadiths:     4000,
				Books:       37,
				Description: "Collection of hadith on jurisprudence",
				Grade:       "Sahih",
			},
			{
				Name:        "riyadussaliheen",
				Title:       "Riyad as-Salihin",
				Author:      "Imam an-Nawawi",
				Hadiths:     1896,
				Books:       20,
				Description: "The Meadows of the Righteous",
				Grade:       "Hasan",
			},
		},
		Books:   make(map[string][]models.Book),
		Hadiths: make(map[string][]models.Hadith),
	}
}

