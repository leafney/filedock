package i18n

import (
	"sort"
	"strconv"
	"strings"
)

type Locale string

const (
	LocaleZhCN    Locale = "zh-CN"
	LocaleEn      Locale = "en"
	DefaultLocale        = LocaleZhCN

	maxLanguageHeaderLength = 512
)

var supportedLocales = []Locale{LocaleZhCN, LocaleEn}

func SupportedLocales() []Locale {
	locales := make([]Locale, len(supportedLocales))
	copy(locales, supportedLocales)
	return locales
}

func Normalize(value string) (Locale, bool) {
	tag := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
	switch {
	case tag == "zh",
		tag == "zh-cn", strings.HasPrefix(tag, "zh-cn-"),
		tag == "zh-sg", strings.HasPrefix(tag, "zh-sg-"),
		tag == "zh-hans", strings.HasPrefix(tag, "zh-hans-"):
		return LocaleZhCN, true
	case tag == "en", strings.HasPrefix(tag, "en-"):
		return LocaleEn, true
	default:
		return "", false
	}
}

type languagePreference struct {
	locale Locale
	weight float64
	order  int
}

func ParseAcceptLanguage(header string) Locale {
	header = strings.TrimSpace(header)
	if header == "" || len(header) > maxLanguageHeaderLength {
		return DefaultLocale
	}

	preferences := make([]languagePreference, 0)
	for order, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		languageRange := strings.TrimSpace(parts[0])
		if languageRange == "" || languageRange == "*" {
			continue
		}

		weight := 1.0
		valid := true
		for _, parameter := range parts[1:] {
			parameter = strings.TrimSpace(parameter)
			name, value, found := strings.Cut(parameter, "=")
			if !found || !strings.EqualFold(strings.TrimSpace(name), "q") {
				valid = false
				break
			}
			parsedWeight, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || parsedWeight < 0 || parsedWeight > 1 {
				valid = false
				break
			}
			weight = parsedWeight
		}
		if !valid || weight == 0 {
			continue
		}

		locale, ok := Normalize(languageRange)
		if !ok {
			continue
		}
		preferences = append(preferences, languagePreference{locale: locale, weight: weight, order: order})
	}

	if len(preferences) == 0 {
		return DefaultLocale
	}
	sort.SliceStable(preferences, func(i, j int) bool {
		if preferences[i].weight == preferences[j].weight {
			return preferences[i].order < preferences[j].order
		}
		return preferences[i].weight > preferences[j].weight
	})
	return preferences[0].locale
}
