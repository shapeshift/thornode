//go:build ignore
// +build ignore

package main

import (
	"os"
	"strings"
	"text/template"

	"github.com/rs/zerolog/log"
)

func main() {
	// template functions
	funcMap := template.FuncMap{
		"ToUpper": strings.ToUpper,
	}

	// read template file
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob("*.tmpl"))

	// open signable_gen.go to write
	f, err := os.Create("signable_gen.go")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create signable_gen.go")
	}
	defer f.Close()
	f.WriteString("// Code generated by go generate; DO NOT EDIT.\n\n")

	// template data
	data := map[string]string{
		"doge": "github.com/eager7/dogd",
		"btc":  "github.com/btcsuite/btcd",
	}

	err = tmpl.ExecuteTemplate(f, "signable.tmpl", data)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to execute template")
	}

	// open wire_gen.go to write
	f, err = os.Create("wire_gen.go")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create wire_gen.go")
	}
	defer f.Close()
	f.WriteString("// Code generated by go generate; DO NOT EDIT.\n\n")

	// template data
	data = map[string]string{
		"doge": "github.com/eager7/dogd",
	}

	err = tmpl.ExecuteTemplate(f, "wire.tmpl", data)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to execute template")
	}
}
