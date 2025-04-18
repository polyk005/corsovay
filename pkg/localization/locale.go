package localization

import (
	"encoding/json"
	"os"
)

type Locale struct {
	translations map[string]string
}

func NewLocale(filePath string) (*Locale, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var translations map[string]string
	if err := json.NewDecoder(file).Decode(&translations); err != nil {
		return nil, err
	}

	return &Locale{translations: translations}, nil
}

func (l *Locale) Translate(key string) string {
	if translation, ok := l.translations[key]; ok {
		return translation
	}
	return key
}
