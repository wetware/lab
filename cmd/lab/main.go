package main

import (
	"fmt"
	"os"

	"github.com/lthibault/log"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"github.com/wetware/lab/internal/cmd"
)

var logger cmd.Logger

var flags = []cli.Flag{
	&cli.StringFlag{
		Name:    "logfmt",
		Aliases: []string{"f"},
		Usage:   "text, json, none",
		Value:   "text",
		EnvVars: []string{"LAB_LOGFMT"},
	},
	&cli.StringFlag{
		Name:    "loglvl",
		Usage:   "trace, debug, info, warn, error, fatal",
		Value:   "info",
		EnvVars: []string{"LAB_LOGLVL"},
	},
}

func main() {
	app := &cli.App{
		Name:   "lab",
		Usage:  "simulation and visualisation tools for CASM researchers",
		Before: setUp,
		Flags:  flags,
		Commands: []*cli.Command{
			cmd.Start(&logger),
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		if exit, ok := err.(cli.ExitCoder); ok {
			os.Exit(exit.ExitCode())
		}

		os.Exit(1)
	}
}

func setUp(c *cli.Context) error {
	logger.Logger = log.New(withLevel(c), withFormat(c))
	return nil
}

// withLevel returns a log.Option that configures a logger's level.
func withLevel(c *cli.Context) (opt log.Option) {
	var level = log.FatalLevel
	defer func() {
		opt = log.WithLevel(level)
	}()

	if c.Bool("trace") {
		level = log.TraceLevel
		return
	}

	if c.String("logfmt") == "none" {
		return
	}

	switch c.String("loglvl") {
	case "trace", "t":
		level = log.TraceLevel
	case "debug", "d":
		level = log.DebugLevel
	case "info", "i":
		level = log.InfoLevel
	case "warn", "warning", "w":
		level = log.WarnLevel
	case "error", "err", "e":
		level = log.ErrorLevel
	case "fatal", "f":
		level = log.FatalLevel
	default:
		level = log.InfoLevel
	}

	return
}

// withFormat returns an option that configures a logger's format.
func withFormat(c *cli.Context) log.Option {
	var fmt logrus.Formatter

	switch c.String("logfmt") {
	case "none":
	case "json":
		fmt = &logrus.JSONFormatter{PrettyPrint: c.Bool("prettyprint")}
	default:
		fmt = new(logrus.TextFormatter)
	}

	return log.WithFormatter(fmt)
}
