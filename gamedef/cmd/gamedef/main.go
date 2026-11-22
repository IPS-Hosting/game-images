// Command gamedef validates and renders IPS Hosting game definitions.
//
//	gamedef validate [-root dir]        validate games/<short>/game.yaml (CI)
//	gamedef render [flags] <game.yaml>  print the start (or install) plan as JSON
//	gamedef schema                      print the JSON schema
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ips-hosting/game-images/gamedef"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "validate":
		err = validate(os.Args[2:])
	case "render":
		err = render(os.Args[2:])
	case "schema":
		_, err = os.Stdout.Write(gamedef.SchemaJSON())
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gamedef validate [-root dir] | render [-set key=value] [-install] [-validate] <game.yaml> | schema")
	os.Exit(2)
}

func validate(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	root := flags.String("root", ".", "repository root containing games/")
	_ = flags.Parse(args)

	files, err := filepath.Glob(filepath.Join(*root, "games", "*", "game.yaml"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no games/*/game.yaml found")
	}
	sort.Strings(files)

	failed := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(*root, file)
		definition, err := gamedef.Parse(data, gamedef.Strict)
		if err == nil {
			if dir := filepath.Base(filepath.Dir(file)); definition.Short != dir {
				err = fmt.Errorf("short %q must equal the directory name %q", definition.Short, dir)
			}
		}
		if err != nil {
			failed++
			for _, line := range strings.Split(err.Error(), "\n") {
				// GitHub Actions annotation format, readable in a terminal too.
				fmt.Printf("::error file=%s::%s\n", rel, line)
			}
			continue
		}
		fmt.Printf("ok %s\n", rel)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d definition(s) invalid", failed, len(files))
	}
	fmt.Printf("%d definition(s) valid\n", len(files))
	return nil
}

type settingFlags map[string]string

func (s settingFlags) String() string { return "" }

func (s settingFlags) Set(value string) error {
	key, val, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("expected key=value, got %q", value)
	}
	s[key] = val
	return nil
}

func render(args []string) error {
	flags := flag.NewFlagSet("render", flag.ExitOnError)
	settings := settingFlags{}
	flags.Var(settings, "set", "setting value key=value (repeatable)")
	install := flags.Bool("install", false, "render the install plan instead of the start plan")
	validateFiles := flags.Bool("validate", false, "with -install: request file validation")
	_ = flags.Parse(args)
	if flags.NArg() != 1 {
		usage()
	}

	data, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		return err
	}
	definition, err := gamedef.Parse(data, gamedef.Strict)
	if err != nil {
		return err
	}
	values := gamedef.DefaultValues(definition)
	for key, value := range settings {
		values.Settings[key] = value
	}

	var plan any
	if *install {
		plan, err = gamedef.RenderInstall(definition, values, gamedef.InstallOptions{Validate: *validateFiles})
	} else {
		plan, err = gamedef.Render(definition, values)
	}
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(plan)
}
