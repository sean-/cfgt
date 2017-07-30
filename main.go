package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/pkg/errors"
)

type GlobalConfig struct {
	Debug       bool
	InFilename  string
	OutFilename string
	OutFormat   string
}

func ConfigureGlobals(app *kingpin.Application) (*GlobalConfig, error) {
	cfg := &GlobalConfig{}

	app.Flag("debug", "Enable debug mode.").
		Short('d').
		BoolVar(&cfg.Debug)
	app.Flag("in", "Filename to read from").
		Short('i').
		Default("-").
		StringVar(&cfg.InFilename)
	app.Flag("out", "Filename to write to").
		Short('o').
		Default("-").
		StringVar(&cfg.OutFilename)

	return cfg, nil
}

func configureApp(app *kingpin.Application) error {
	var err error
	var globals *GlobalConfig
	globals, err = ConfigureGlobals(app)
	if err != nil {
		return errors.Wrap(err, "unable to configure globals")
	}

	if err := ConfigureParseCommand(app, globals); err != nil {
		return errors.Wrap(err, "unable to configure parse command")
	}

	return nil
}

func main() {
	app := kingpin.New("cfgt", "A configuration file translation utility")
	app.Author("Joyent, Inc.")
	app.Version("0.1")
	if err := configureApp(app); err != nil {
		type stackTracer interface {
			StackTrace() errors.StackTrace
		}

		switch origErr := err.(type) {
		case stackTracer:
			fmt.Printf("Bad: %v\n", origErr)
			for _, f := range origErr.StackTrace() {
				fmt.Printf("%+v\n", f)
			}
		default:
			fmt.Printf("Bad: %v\n", err)
		}
	}
	kingpin.MustParse(app.Parse(os.Args[1:]))
}
