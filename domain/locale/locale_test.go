package locale

import (
	"testing"
	"time"
)

func TestLocalePrecedence(t *testing.T) {
	t.Cleanup(func() { SetLocale("") })
	t.Setenv("NITRO_LOCALE", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "pt_BR.UTF-8")
	if got := Locale(); got != "pt-BR" {
		t.Fatalf("LANG: got %q", got)
	}
	t.Setenv("LANG", "C.UTF-8")
	if got := Locale(); got != "en-US" {
		t.Fatalf("POSIX C locale must fall back to en-US, got %q", got)
	}
	t.Setenv("NITRO_LOCALE", "es_ar")
	if got := Locale(); got != "es-AR" {
		t.Fatalf("NITRO_LOCALE: got %q", got)
	}
	SetLocale("fr")
	if got := Locale(); got != "fr" {
		t.Fatalf("override: got %q", got)
	}
	if WithLocale("de_DE") != "de-DE" || WithLocale(" ") != "fr" {
		t.Fatal("WithLocale")
	}
}

func TestTimezone(t *testing.T) {
	t.Cleanup(func() {
		mu.Lock()
		tzLoc = nil
		mu.Unlock()
	})
	t.Setenv("NITRO_TZ", "America/Sao_Paulo")
	if Timezone().String() != "America/Sao_Paulo" {
		t.Fatalf("NITRO_TZ: %v", Timezone())
	}
	if err := SetTimezone("Europe/Lisbon"); err != nil {
		t.Fatal(err)
	}
	if Timezone().String() != "Europe/Lisbon" {
		t.Fatalf("SetTimezone: %v", Timezone())
	}
	if err := SetTimezone("Not/AZone"); err == nil {
		t.Fatal("invalid zone must fail")
	}
	_ = SetTimezone("")
	if Timezone() != time.UTC {
		t.Fatal(`SetTimezone("") must pin UTC`)
	}
}
