package transform

import (
	"strings"
	"unicode"
)

// foldTable maps accented Latin letters to ASCII (Portuguese, Spanish,
// French, German, Nordic and Central European cover the common cases).
var foldTable = map[rune]string{
	'á': "a", 'à': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ė': "e", 'ę': "e", 'ě': "e",
	'í': "i", 'ì': "i", 'î': "i", 'ï': "i", 'ī': "i", 'į': "i", 'ı': "i",
	'ó': "o", 'ò': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ő': "o",
	'ú': "u", 'ù': "u", 'û': "u", 'ü': "u", 'ū': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'ý': "y", 'ÿ': "y", 'ç': "c", 'ć': "c", 'č': "c", 'ñ': "n", 'ń': "n", 'ň': "n",
	'ś': "s", 'š': "s", 'ş': "s", 'ź': "z", 'ż': "z", 'ž': "z", 'ł': "l", 'ď': "d", 'đ': "d",
	'ť': "t", 'ř': "r", 'ğ': "g", 'ß': "ss", 'æ': "ae", 'œ': "oe", 'þ': "th", 'ð': "d",
}

// FoldAccents replaces accented Latin letters with their ASCII base
// ("São Paulo" → "Sao Paulo"). Other runes are kept.
func FoldAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		lower := unicode.ToLower(r)
		if f, ok := foldTable[lower]; ok {
			if lower != r {
				f = strings.ToUpper(f[:1]) + f[1:]
			}
			b.WriteString(f)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Slugify returns a URL-safe, lowercase, hyphen-separated slug:
// "Café & Cia. São Paulo!" → "cafe-cia-sao-paulo".
func Slugify(s string) string {
	s = FoldAccents(s)
	var b strings.Builder
	b.Grow(len(s))
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}
