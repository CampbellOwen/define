// Copyright © 2018 Trevor N. Suarez (Rican7)

// A command-line dictionary (thesaurus) app, written in Go.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Rican7/define/internal/action"
	"github.com/Rican7/define/internal/config"
	defineio "github.com/Rican7/define/internal/io"
	"github.com/Rican7/define/internal/io/printer"
	"github.com/Rican7/define/internal/version"
	"github.com/Rican7/define/registry"
	"github.com/Rican7/define/source"
	flag "github.com/ogier/pflag"

	"github.com/Rican7/define/source/glosbe"
	_ "github.com/Rican7/define/source/oxford"
	_ "github.com/Rican7/define/source/webster"
)

const (
	// Configuration defaults
	defaultConfigFileLocation = "~/.define.conf.json"
	defaultIndentationSize    = 2
	defaultPreferredSource    = glosbe.JSONKey
)

var (
	stdErrWriter = defineio.NewPanicWriter(os.Stderr, defaultIndentationSize)
	stdOutWriter = defineio.NewPanicWriter(os.Stdout, defaultIndentationSize)

	flags *flag.FlagSet
	act   *action.Action
	conf  config.Configuration
	src   source.Source
)

func init() {
	var err error

	flags = flag.NewFlagSet(version.AppName, flag.ContinueOnError)
	flags.SetOutput(stdErrWriter)
	flags.Usage = func() {
		printUsage(stdErrWriter)
		quit(2)
	}

	act = action.Setup(flags)

	// Configure our registered providers
	providerConfs := registry.ConfigureProviders(flags)

	if len(providerConfs) < 1 {
		handleError(fmt.Errorf("no registered source providers"))
	}

	conf, err = config.NewFromRuntime(flags, providerConfs, defaultConfigFileLocation, config.Configuration{
		IndentationSize: defaultIndentationSize,
		PreferredSource: defaultPreferredSource,
	})

	// Re-initialize our writers once we have our indentation size configuration
	stdErrWriter = defineio.NewPanicWriter(os.Stderr, conf.IndentationSize)
	stdOutWriter = defineio.NewPanicWriter(os.Stdout, conf.IndentationSize)
	flags.SetOutput(stdErrWriter)

	// Finalize our configurations
	for _, providerConf := range providerConfs {
		registry.Finalize(providerConf)
	}

	handleError(err)

	var preferredProviderConfig registry.Configuration

	if "" != conf.PreferredSource {
		if providerConf, ok := providerConfs[conf.PreferredSource]; ok {
			preferredProviderConfig = providerConf
		} else {
			handleError(fmt.Errorf("preferred provider/source %q does not exist", conf.PreferredSource))
		}
	} else {
		for _, providerConf := range providerConfs {
			preferredProviderConfig = providerConf
			break
		}
	}

	src, err = registry.Provide(preferredProviderConfig)

	if nil != err {
		handleError(
			fmt.Errorf(
				"source %q failed to initialize with error: %s",
				registry.ProviderName(preferredProviderConfig),
				err,
			),
		)
	}

	// Make sure our flags are parsed before entering main
	handleError(flags.Parse(os.Args[1:]))
}

func handleError(err ...error) {
	for _, e := range err {
		if nil != e {
			msg := e.Error()

			if len(msg) > 1 {
				// Format the message
				msg = strings.ToTitle(msg[:1]) + msg[1:]

				stdErrWriter.IndentWrites(func(writer *defineio.PanicWriter) {
					writer.WritePaddedStringLine(msg, 1)
				})
			}

			quit(1)
		}
	}
}

func quit(code int) {
	os.Exit(code)
}

func printConfig() {
	encoded, err := json.MarshalIndent(conf, "", "    ")

	handleError(err)

	stdOutWriter.WriteStringLine(string(encoded))
}

func printSources() {
	stdOutWriter.IndentWrites(func(writer *defineio.PanicWriter) {
		writer.WritePaddedStringLine("Available sources:", 1)

		for i, source := range registry.ProviderNames() {
			writer.WriteStringLine(fmt.Sprintf("%d. %q", i+1, source))
		}

		writer.WriteNewLine()
	})
}

func printVersion() {
	stdOutWriter.WriteStringLine(version.Printable())
}

func printUsage(writer *defineio.PanicWriter) {
	writer.IndentWrites(func(w *defineio.PanicWriter) {
		flags.SetOutput(w)

		w.WritePaddedStringLine(fmt.Sprintf("Usage: %s [<options>...] <word>", version.AppName), 1)

		w.WriteStringLine("Options:")
		flags.PrintDefaults()
		w.WriteNewLine()
	})
}

func defineWord(word string) {
	result, err := src.Define(word)

	handleError(err, source.ValidateResult(result))

	resultPrinter := printer.NewResultPrinter(stdOutWriter)

	resultPrinter.PrintResult(result)
	resultPrinter.PrintSourceName(src)
}

func main() {
	// Get the word from our first non-flag argument
	word := flags.Arg(0)

	// Decide what to perform
	switch act.Type() {
	case action.PrintConfig:
		printConfig()
	case action.ListSources:
		printSources()
	case action.PrintVersion:
		printVersion()
	case action.DefineWord:
		fallthrough
	default:
		if "" == word {
			// Show our usage
			printUsage(stdOutWriter)
			quit(1)
		} else {
			defineWord(word)
		}
	}
}
