package panel

import "testing"

func TestMatchLocale(t *testing.T) {
	cases := []struct {
		name      string
		preferred []string
		want      Locale
	}{
		{
			name:      "simplified chinese first",
			preferred: []string{"zh-Hans-CN", "en-US"},
			want:      LocaleZhCN,
		},
		{
			name:      "english first",
			preferred: []string{"en-US", "zh-Hans-CN"},
			want:      LocaleEn,
		},
		{
			name:      "traditional chinese skips to simplified chinese",
			preferred: []string{"zh-Hant-TW", "zh-Hans"},
			want:      LocaleZhCN,
		},
		{
			name:      "traditional chinese skips to english",
			preferred: []string{"zh-Hant-TW", "en-US"},
			want:      LocaleEn,
		},
		{
			name:      "traditional chinese alone falls back to english",
			preferred: []string{"zh-Hant-TW"},
			want:      LocaleEn,
		},
		{
			name:      "hong kong chinese skips to english",
			preferred: []string{"zh-HK"},
			want:      LocaleEn,
		},
		{
			name:      "macau chinese skips to english",
			preferred: []string{"zh-MO"},
			want:      LocaleEn,
		},
		{
			name:      "singapore chinese matches simplified",
			preferred: []string{"zh-SG"},
			want:      LocaleZhCN,
		},
		{
			name:      "bare zh matches simplified",
			preferred: []string{"zh"},
			want:      LocaleZhCN,
		},
		{
			name:      "unsupported first falls through to simplified chinese",
			preferred: []string{"ja-JP", "zh-Hans-CN"},
			want:      LocaleZhCN,
		},
		{
			name:      "unsupported languages fall back to english",
			preferred: []string{"fr-FR", "de-DE"},
			want:      LocaleEn,
		},
		{
			name:      "underscore tag normalized",
			preferred: []string{"zh_CN"},
			want:      LocaleZhCN,
		},
		{
			name:      "empty list falls back to english",
			preferred: nil,
			want:      LocaleEn,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchLocale(tc.preferred); got != tc.want {
				t.Fatalf("MatchLocale(%v) = %q, want %q", tc.preferred, got, tc.want)
			}
		})
	}
}

func TestEnvLocale(t *testing.T) {
	t.Run("prefers LC_ALL over LANG", func(t *testing.T) {
		t.Setenv("LC_ALL", "en_US.UTF-8")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "zh_CN.UTF-8")
		if got := envLocale(); got != LocaleEn {
			t.Fatalf("envLocale() = %q, want %q", got, LocaleEn)
		}
	})

	t.Run("ignores C and POSIX and falls through to LANG", func(t *testing.T) {
		t.Setenv("LC_ALL", "C")
		t.Setenv("LC_MESSAGES", "POSIX")
		t.Setenv("LANG", "zh_CN.UTF-8")
		if got := envLocale(); got != LocaleZhCN {
			t.Fatalf("envLocale() = %q, want %q", got, LocaleZhCN)
		}
	})

	t.Run("defaults to english when unset", func(t *testing.T) {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "")
		if got := envLocale(); got != LocaleEn {
			t.Fatalf("envLocale() = %q, want %q", got, LocaleEn)
		}
	})
}

func TestLocaleIsZh(t *testing.T) {
	if !LocaleZhCN.IsZh() {
		t.Fatal("LocaleZhCN.IsZh() = false")
	}
	if LocaleEn.IsZh() {
		t.Fatal("LocaleEn.IsZh() = true")
	}
	if Locale("").IsZh() {
		t.Fatal("Locale(\"\").IsZh() = true")
	}
}
