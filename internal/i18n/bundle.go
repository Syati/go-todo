package i18n

import (
	"embed"
	"encoding/json"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed messages.*.json
var messages embed.FS

type Bundle struct {
	bundle *goi18n.Bundle
}

type Localizer struct {
	localizer *goi18n.Localizer
	language  string
}

func NewBundle() (*Bundle, error) {
	bundle := goi18n.NewBundle(language.Japanese)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, name := range []string{"messages.ja.json", "messages.en.json"} {
		if _, err := bundle.LoadMessageFileFS(messages, name); err != nil {
			return nil, err
		}
	}
	return &Bundle{bundle: bundle}, nil
}

func (b *Bundle) For(acceptLanguage string) Localizer {
	locale := localeFor(acceptLanguage)
	return Localizer{
		localizer: goi18n.NewLocalizer(b.bundle, locale),
		language:  locale,
	}
}

func (l Localizer) Language() string { return l.language }

func (l Localizer) Message(id string) string {
	message, err := l.localizer.Localize(&goi18n.LocalizeConfig{MessageID: id})
	if err != nil {
		return id
	}
	return message
}

func localeFor(acceptLanguage string) string {
	if strings.TrimSpace(acceptLanguage) == "" {
		return "ja"
	}
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil {
		return "en"
	}
	for _, tag := range tags {
		base, _ := tag.Base()
		switch base.String() {
		case "ja":
			return "ja"
		case "en":
			return "en"
		}
	}
	return "en"
}
