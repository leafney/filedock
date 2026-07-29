package i18n

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		value  string
		want   Locale
		wantOK bool
	}{
		{value: "zh", want: LocaleZhCN, wantOK: true},
		{value: "ZH_cn", want: LocaleZhCN, wantOK: true},
		{value: "zh-CN-u-nu-hanidec", want: LocaleZhCN, wantOK: true},
		{value: "zh-Hans-SG", want: LocaleZhCN, wantOK: true},
		{value: "en-US", want: LocaleEn, wantOK: true},
		{value: "zh-TW", wantOK: false},
		{value: "fr", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, ok := Normalize(tt.value)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("Normalize(%q) = (%q, %v), want (%q, %v)", tt.value, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   Locale
	}{
		{name: "empty", want: LocaleZhCN},
		{name: "english", header: "en", want: LocaleEn},
		{name: "english region", header: "en-US", want: LocaleEn},
		{name: "quality", header: "zh-CN;q=0.4, en-US;q=0.9", want: LocaleEn},
		{name: "same quality keeps order", header: "en, zh-CN", want: LocaleEn},
		{name: "unsupported first", header: "fr, en;q=0.8", want: LocaleEn},
		{name: "traditional chinese then english", header: "zh-TW, en;q=0.8", want: LocaleEn},
		{name: "wildcard", header: "*", want: LocaleZhCN},
		{name: "zero quality", header: "en;q=0", want: LocaleZhCN},
		{name: "invalid quality", header: "en;q=two", want: LocaleZhCN},
		{name: "unknown parameter", header: "en;level=1", want: LocaleZhCN},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseAcceptLanguage(tt.header); got != tt.want {
				t.Fatalf("ParseAcceptLanguage(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}
