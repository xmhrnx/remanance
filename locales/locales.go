// Package locales holds the app's text, one file per language: en.json
// for English, which is also what shows when a language lacks a message.
// It loads them as it starts, so every screen and test that reads text
// finds it.
//
// To add a language, copy en.json to its code (fr.json, pt-br.json),
// translate the values and leave the keys. {name} placeholders are filled
// in by the app; a message with "one" and "other" forms is chosen by
// count.
package locales

import (
	"embed"

	"github.com/mehran9675/giode/i18n"
)

//go:embed *.json
var files embed.FS

func init() {
	if err := i18n.Load(files, "."); err != nil {
		panic("locales: " + err.Error())
	}
	i18n.SetFallback("en")
}

// T is the message for key in the user's language, with {name}
// placeholders filled from name, value pairs.
func T(key string, args ...any) string { return i18n.T(key, args...) }

// N is the plural form of key for n, with {n} and any other
// placeholders filled.
func N(key string, n int, args ...any) string { return i18n.N(key, n, args...) }

// Label is an English name from the engines, such as a scan's phase, in
// the user's language when there's a message for it under label., or as
// it is.
func Label(s string) string {
	key := "label." + s
	if t := i18n.T(key); t != key {
		return t
	}
	return s
}
