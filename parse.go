package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/flynn/json5"
	"github.com/hashicorp/hcl"
	isatty "github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

const (
	DefaultBufferSize = 16384
)

type ParseMode struct {
	*GlobalConfig
	BufferSize   int
	PrettyPrint  bool
	InputFormat  string
	OutputFormat string
}

func NewParseMode(globals *GlobalConfig) ParseMode {
	return ParseMode{
		GlobalConfig: globals,
		BufferSize:   DefaultBufferSize,
	}
}

func ConfigureParseCommand(app *kingpin.Application, globals *GlobalConfig) error {
	parseMode := NewParseMode(globals)

	parseCommand := app.Command("convert", "Convert a configuration file to JSON").
		Default().
		Action(parseMode.run)

	parseCommand.Flag("in-format", `Input format ("json", "json5", or "hcl")`).
		Short('I').
		Default("*").
		StringVar(&parseMode.InputFormat)

	parseCommand.Flag("out-format", `Output format ("json")`).
		Short('O').
		Default("json").
		StringVar(&parseMode.OutputFormat)

	parseCommand.Flag("pretty", "Pretty-print the output (true if output is a terminal)").
		Short('p').
		Default(fmt.Sprintf("%t", isatty.IsTerminal(os.Stdout.Fd()))).
		BoolVar(&parseMode.PrettyPrint)

	return nil
}

func (m *ParseMode) run(c *kingpin.ParseContext) error {
	var f *os.File
	var err error
	if m.InFilename == "-" {
		// When the input is stdin, write the input to a tempfile so in the event of
		// an error highlightPosition() can scan the file to provide a useful hint
		// regarding the syntax error.
		f, err = ioutil.TempFile(os.TempDir(), "json5")
		if err != nil {
			return errors.Wrap(err, "unable to create temp file for stdin")
		}
		defer os.Remove(f.Name())
		defer f.Close()

		w := bufio.NewWriterSize(f, m.BufferSize)
		io.Copy(w, bufio.NewReaderSize(os.Stdin, m.BufferSize))
		err = w.Flush()
		if err != nil {
			return errors.Wrap(err, "unable to flush temp file")
		}

		f.Seek(0, os.SEEK_SET)
	} else {
		var err error
		f, err = os.Open(m.InFilename)
		if err != nil {
			return errors.Wrap(err, "unable to read input")
		}
		defer f.Close()
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		return errors.Wrap(err, "unable to read input")
	}

	var raw interface{}
	var tryAllFormats bool
	errList := make([]error, 0, 3)
	switch m.InputFormat {
	case "*":
		tryAllFormats = true
		fallthrough
	case "json":
		raw, err = ParseJSON(strings.NewReader(string(buf.Bytes())))
		if err == nil {
			break
		}

		var errWrapped error
		switch parseErr := errors.Cause(err).(type) {
		case *json.SyntaxError:
			f.Seek(0, os.SEEK_SET)
			// Grab the error location, and return a string to point to offending syntax error
			line, col, highlight := highlightPosition(f, parseErr.Offset)
			errWrapped = errors.Wrapf(err, "unable to parse %q as %q: %s\nSyntax error at line %d, column %d (offset %d):\n%s", m.InFilename, m.InputFormat, parseErr, line, col, parseErr.Offset, highlight)
		default:
			errWrapped = errors.Wrapf(err, "unable to parse config file as %q", m.InputFormat)
		}

		if !tryAllFormats {
			return errWrapped
		} else {
			errList = append(errList, errWrapped)
		}

		fallthrough
	case "json5":
		raw, err = ParseJSON5(strings.NewReader(string(buf.Bytes())))
		if err == nil {
			break
		}

		var errWrapped error
		switch parseErr := errors.Cause(err).(type) {
		case *json5.SyntaxError:
			f.Seek(0, os.SEEK_SET)
			// Grab the error location, and return a string to point to offending syntax error
			line, col, highlight := highlightPosition(f, parseErr.Offset)
			errWrapped = errors.Wrapf(err, "unable to parse %q as %q: %s\nSyntax error at line %d, column %d (offset %d):\n%s", m.InFilename, m.InputFormat, parseErr, line, col, parseErr.Offset, highlight)
		default:
			errWrapped = errors.Wrapf(err, "unable to parse config file as %q", m.InputFormat)
		}

		if !tryAllFormats {
			return errWrapped
		} else {
			errList = append(errList, errWrapped)
		}

		fallthrough
	case "hcl":
		raw, err = ParseHCL(string(buf.Bytes()))
		if err == nil {
			break
		}

		var errWrapped error
		switch parseErr := errors.Cause(err).(type) {
		default:
			_ = parseErr // Preserve structure for future and improved error handling
			errWrapped = errors.Wrapf(err, "unable to parse config file as %q", m.InputFormat)
		}

		if !tryAllFormats {
			return errWrapped
		} else {
			errList = append(errList, errWrapped)
		}

		fallthrough
	default:
		if len(errList) > 0 {
			return fmt.Errorf("Unsupported input type: %q: %v", m.InputFormat, errList)
		} else {
			return fmt.Errorf("Unsupported input type: %q", m.InputFormat)
		}
	}

	var w *bufio.Writer
	switch m.OutFilename {
	case "-":
		w = bufio.NewWriterSize(os.Stdout, m.BufferSize)
	default:
		// Assume a file
		fo, err := os.Create(m.OutFilename)
		if err != nil {
			return errors.Wrap(err, "unable to open output file")
		}

		// FIXME (seanc@): Need to not panic() in a defer
		defer func() {
			if err := fo.Close(); err != nil {
				panic(err)
			}
		}()

		// FIXME (seanc@): Need to not panic() in a defer
		defer func() {
			if err := fo.Sync(); err != nil {
				panic(err)
			}
		}()

		w = bufio.NewWriter(fo)
	}

	defer w.Flush()

	switch m.OutputFormat {
	case "json":
		enc := json.NewEncoder(w)

		if m.PrettyPrint && m.OutFilename == "-" {
			enc.SetIndent("", "    ")
		}

		if err = enc.Encode(raw); err != nil {
			return errors.Wrap(err, "unable to encode")
		}
	default:
		return fmt.Errorf("Unsupported output type: %q", m.OutputFormat)
	}

	return nil
}

// ParseHCL takes the given io.Reader and parses a Template object out of it.
func ParseHCL(input string) (interface{}, error) {
	var raw interface{}
	if err := hcl.Decode(&raw, input); err != nil {
		return nil, errors.Wrap(err, "unable to decode HCL")
	}

	return raw, nil
}

// ParseJSON takes the given io.Reader and parses a Template object out of it.
func ParseJSON(r io.Reader) (interface{}, error) {
	// Create a buffer to copy what we read
	// var buf bytes.Buffer
	// r = io.TeeReader(r, &buf)

	// First, decode the object into an interface{}. We do this instead of
	// the rawTemplate directly because we'd rather use mapstructure to
	// decode since it has richer errors.
	var raw interface{}
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, errors.Wrap(err, "unable to decode JSON")
	}

	return raw, nil
}

// ParseJSON5 takes the given io.Reader and parses a Template object out of it.
func ParseJSON5(r io.Reader) (interface{}, error) {
	// Create a buffer to copy what we read
	// var buf bytes.Buffer
	// r = io.TeeReader(r, &buf)

	// First, decode the object into an interface{}. We do this instead of
	// the rawTemplate directly because we'd rather use mapstructure to
	// decode since it has richer errors.
	var raw interface{}
	if err := json5.NewDecoder(r).Decode(&raw); err != nil {
		return nil, errors.Wrap(err, "unable to decode JSON5")
	}

	return raw, nil
}

// Takes a file and the location in bytes of a parse error from
// json5.SyntaxError.Offset and returns the line, column, and pretty-printed
// context around the error with an arrow indicating the exact position of the
// syntax error.
func highlightPosition(f *os.File, pos int64) (line, col int, highlight string) {
	// Modified version of the function in Camlistore by Brad Fitzpatrick
	// https://github.com/camlistore/camlistore/blob/4b5403dd5310cf6e1ae8feb8533fd59262701ebc/vendor/go4.org/errorutil/highlight.go
	line = 1
	br := bufio.NewReader(f)
	lastLine := ""
	thisLine := new(bytes.Buffer)
	for n := int64(0); n < pos; n++ {
		b, err := br.ReadByte()
		if err != nil {
			break
		}
		if b == '\n' {
			lastLine = thisLine.String()
			thisLine.Reset()
			line++
			col = 1
		} else {
			col++
			thisLine.WriteByte(b)
		}
	}

	if line > 1 {
		highlight += fmt.Sprintf("%5d: %s\n", line-1, lastLine)
	}

	highlight += fmt.Sprintf("%5d: %s\n", line, thisLine.String())
	highlight += fmt.Sprintf("%s^\n", strings.Repeat(" ", col+5))
	return
}
