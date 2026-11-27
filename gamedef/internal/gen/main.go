// Command gen writes the JSON files generated from the Go tables (run via go generate in
// the gamedef directory).
package main

import (
	"log"
	"os"

	"github.com/ips-hosting/game-images/gamedef"
)

func main() {
	files := map[string][]byte{
		"schema/engine-features.v1.json": gamedef.EngineFeaturesJSON(),
		"schema/categories.v1.json":      gamedef.CategoriesJSON(),
	}
	for name, content := range files {
		if err := os.WriteFile(name, content, 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
