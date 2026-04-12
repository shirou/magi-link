package spell

import (
	"embed"
	"fmt"
	"regexp"

	"github.com/BurntSushi/toml"
)

// validLang matches safe locale identifiers (e.g., "en", "ja", "pt-BR").
var validLang = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z]{2,4})?$`)

//go:embed locales
var embeddedLocales embed.FS

const DefaultLang = "ja"

// SpellText holds localized display strings for a single spell.
type SpellText struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

// Locale maps spell IDs to localized text with optional fallback.
type Locale struct {
	lang     string
	texts    map[string]SpellText
	fallback *Locale
}

// LoadLocale loads a locale from the embedded locales directory.
// It automatically sets DefaultLang ("ja") as the fallback unless
// the requested language is already the default.
func LoadLocale(lang string) (*Locale, error) {
	loc, err := loadSingleLocale(lang)
	if err != nil {
		return nil, err
	}

	if lang != DefaultLang {
		fb, err := loadSingleLocale(DefaultLang)
		if err != nil {
			return nil, fmt.Errorf("load fallback locale %q: %w", DefaultLang, err)
		}
		loc.fallback = fb
	}

	return loc, nil
}

func loadSingleLocale(lang string) (*Locale, error) {
	if !validLang.MatchString(lang) {
		return nil, fmt.Errorf("invalid locale identifier %q", lang)
	}
	path := fmt.Sprintf("locales/%s.toml", lang)
	data, err := embeddedLocales.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read locale %q: %w", lang, err)
	}

	var texts map[string]SpellText
	if err := toml.Unmarshal(data, &texts); err != nil {
		return nil, fmt.Errorf("parse locale %q: %w", lang, err)
	}

	return &Locale{lang: lang, texts: texts}, nil
}

// Lang returns the language code of this locale.
func (l *Locale) Lang() string {
	return l.lang
}

// Text returns the localized text for a spell ID.
// Falls back to the fallback locale, then to the spell ID itself.
func (l *Locale) Text(spellID string) SpellText {
	if t, ok := l.texts[spellID]; ok {
		return t
	}
	if l.fallback != nil {
		return l.fallback.Text(spellID)
	}
	return SpellText{Name: spellID, Description: ""}
}
