package main

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed templates/*
var projectTemplateFS embed.FS

func readTemplate(name string) string {
	b, err := projectTemplateFS.ReadFile("templates/" + name)
	if err != nil {
		panic("nitrogen: missing template " + name + ": " + err.Error())
	}
	return strings.TrimSpace(string(b))
}

func sprintfTemplate(name string, args ...any) string {
	return fmt.Sprintf(readTemplate(name), args...)
}
