package connect

import (
	"cursovay/internal/model"
	"time"

	"github.com/google/uuid"
)

type Document struct {
	ID            string
	FilePath      string
	Manufacturers []model.Manufacturer
	SearchQuery   string
	SearchResults []model.Manufacturer
	IsSearching   bool
	Active        bool // Added this field
	Unsaved       bool
	LastModified  time.Time
}

// Add this constructor function
func NewDocument(filePath string, manufacturers []model.Manufacturer) *Document {
	return &Document{
		ID:            uuid.New().String(),
		FilePath:      filePath,
		Manufacturers: manufacturers,
		Active:        true, // New documents are active by default
		Unsaved:       filePath == "",
		LastModified:  time.Now(),
	}
}
