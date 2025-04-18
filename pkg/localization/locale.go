package localization

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Locale struct {
	translations map[string]string
	language     string
}

func NewLocale(localesDir string) (*Locale, error) {
	// Определяем язык (можно сделать параметром)
	lang := "ru" // или "en"

	filePath := filepath.Join(localesDir, lang+".json")

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var translations map[string]string
	if err := json.NewDecoder(file).Decode(&translations); err != nil {
		return nil, err
	}

	return &Locale{
		translations: translations,
		language:     lang,
	}, nil
}

// Добавляем метод Translate
func (l *Locale) Translate(key string) string {
	if translation, ok := l.translations[key]; ok {
		return translation
	}
	return key // Возвращаем ключ, если перевод не найден
}

// Добавляем метод для смены языка
func (l *Locale) SetLanguage(lang string, localesDir string) error {
	filePath := filepath.Join(localesDir, lang+".json")

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var translations map[string]string
	if err := json.NewDecoder(file).Decode(&translations); err != nil {
		return err
	}

	l.translations = translations
	l.language = lang
	return nil
}
