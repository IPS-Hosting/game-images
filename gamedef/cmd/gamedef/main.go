// Command gamedef validates and renders IPS Hosting game definitions.
//
//	gamedef validate [-root dir]        validate games/<short>/game.yaml (CI)
//	gamedef render [flags] <game.yaml>  print the start (or install) plan as JSON
//	gamedef images [-check] [-root dir] list the digest-pinned images; -check verifies them (CI)
//	gamedef schema                      print the JSON schema
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ips-hosting/game-images/gamedef"
	"github.com/ips-hosting/game-images/gamedef/schema"
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
	case "images":
		err = images(os.Args[2:])
	case "schema":
		_, err = os.Stdout.Write(schema.JSON())
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: gamedef validate [-root dir] | render [-set key=value] [-install] [-validate] <game.yaml> | images [-check] [-root dir] | schema")
	os.Exit(2)
}

func validate(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	root := flags.String("root", ".", "repository root containing games/")
	_ = flags.Parse(args)

	files, err := catalog(*root)
	if err != nil {
		return err
	}

	failed := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(*root, file)
		definition, err := parseFile(data)
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
	definition, err := parseFile(data)
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

// catalog returns the sorted game.yaml files below root.
func catalog(root string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(root, "games", "*", "game.yaml"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("no games/*/game.yaml found")
	}
	sort.Strings(files)
	return files, nil
}

// parseFile runs the JSON schema check and the strict parse with semantic checks.
func parseFile(data []byte) (*gamedef.Definition, error) {
	if err := schema.ValidateSchema(data); err != nil {
		return nil, err
	}
	return gamedef.Parse(data)
}

// images prints every digest-pinned image reference of the catalog once (input for
// mirroring into the platform registry). With -check it verifies against the registries,
// without pulling, that every pinned digest exists (an index must contain linux/amd64) and
// that every own tag exists; a missing own tag is only a warning.
func images(args []string) error {
	flags := flag.NewFlagSet("images", flag.ExitOnError)
	root := flags.String("root", ".", "repository root containing games/")
	check := flags.Bool("check", false, "check the references against the registries")
	_ = flags.Parse(args)

	files, err := catalog(*root)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var refs []string
	add := func(image string) {
		if !seen[image] && (strings.Contains(image, "@sha256:") || *check) {
			seen[image] = true
			refs = append(refs, image)
		}
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		definition, err := gamedef.Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		for _, image := range definition.Images() {
			add(image)
		}
		for _, stage := range definition.Install {
			if stage.Image == "" {
				add(gamedef.DefaultInstallerImage)
			}
		}
	}
	sort.Strings(refs)
	if !*check {
		for _, image := range refs {
			fmt.Println(image)
		}
		return nil
	}
	return reportImages(os.Stdout, newRegistryChecker().checkAll(context.Background(), refs))
}

// reportImages prints one line per result and fails on any failed check.
func reportImages(out io.Writer, results []imageResult) error {
	counts := map[string]int{}
	for _, result := range results {
		counts[result.Status]++
		switch result.Status {
		case statusFail:
			fmt.Fprintf(out, "::error::%s: %s\n", result.Ref, result.Detail)
		case statusWarn:
			fmt.Fprintf(out, "::warning::%s: %s\n", result.Ref, result.Detail)
		default:
			fmt.Fprintf(out, "ok %s (%s)\n", result.Ref, result.Detail)
		}
	}
	fmt.Fprintf(out, "%d image(s): %d ok, %d warning(s), %d failed\n", len(results), counts[statusOK], counts[statusWarn], counts[statusFail])
	if counts[statusFail] > 0 {
		return fmt.Errorf("%d image check(s) failed", counts[statusFail])
	}
	return nil
}
