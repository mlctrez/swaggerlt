package main

import (
	"fmt"
	"github.com/mlctrez/swaggerlt"
	"os"
	"regexp"
)

func main() {

	options := &swaggerlt.Options{SpecFile: "spec.json", PathRegex: regexp.MustCompile("/v0"),
		ModuleName: "github.com/mlctrez/swaggerlt", ServiceName: "smapi",
	}

	if generator, err := swaggerlt.New(options); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else {
		if err = generator.Execute(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

}
