package swaggerlt

import (
	"fmt"
	"github.com/dave/jennifer/jen"
	"os"
	"path/filepath"
)

func (g *Generator) createClientFile(version string) error {

	nfp := filepath.Join(g.Options.ModuleName, g.Options.ServiceName+version, "client")

	f := jen.NewFilePath(nfp)
	f.HeaderComment("Automatically generated, do not edit!")

	doc := "Client for service %s version %s."
	f.Comment(fmt.Sprintf(doc, g.Options.ServiceName, version))

	f.Type().Id("Client").Struct(
		jen.Id("Client").Op("*").Qual("net/http", "Client"),
		jen.Id("Endpoint").String(),
	)

	versionDir := g.versionDirectory(version)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return err
	}

	clientFilePath := filepath.Join(versionDir, "client", "client.go")
	if err := os.MkdirAll(filepath.Dir(clientFilePath), 0755); err != nil {
		return err
	}

	if create, err := os.Create(clientFilePath); err != nil {
		return err
	} else {
		defer func() { _ = create.Close() }()
		return f.Render(create)
	}

}
