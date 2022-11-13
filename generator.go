package swaggerlt

import (
	"encoding/json"
	"fmt"
	jp "github.com/buger/jsonparser"
	"github.com/dave/jennifer/jen"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Options struct {
	PathRegex   *regexp.Regexp
	SpecFile    string
	ServiceName string
	ModuleName  string
}

func New(options *Options) (*Generator, error) {
	specBytes, err := os.ReadFile(options.SpecFile)
	if err != nil {
		return nil, err
	}
	result := &Generator{
		Options:      options,
		specBytes:    specBytes,
		refGroup:     &sync.WaitGroup{},
		refManager:   make(chan string, 100),
		refChan:      make(chan string, 100),
		refCompleted: map[string]bool{},
	}
	return result, nil
}

type Generator struct {
	Options   *Options
	specBytes []byte

	refGroup     *sync.WaitGroup
	refManager   chan string
	refChan      chan string
	refCompleted map[string]bool
}

func (g *Generator) manager() {
	for ref := range g.refManager {
		if g.refCompleted[ref] {
			// work has already been completed
			g.refGroup.Done()
			continue
		}
		// delegate to workers
		g.refCompleted[ref] = true
		g.refChan <- ref
	}
}

func (g *Generator) refWorker() {
	for ref := range g.refChan {

		// TODO: simplify and break up this long method

		path, name := g.refPathAndType(ref)

		jc := jen.NewFilePath(path)

		outputFile := filepath.Join(
			strings.TrimPrefix(path, g.Options.ModuleName+"/"),
			fmt.Sprintf("%s.go", toGoNameLower(name)),
		)
		err := os.MkdirAll(filepath.Dir(outputFile), 0755)
		if err != nil {
			log.Fatal(err)
		}

		writeOutputFile := func() {
			create, err := os.Create(outputFile)
			if err != nil {
				log.Fatal(err)
			}
			err = jc.Render(create)
			if err != nil {
				log.Fatal(err)
			}
			_ = create.Close()
		}

		parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
		value, _, _, err := jp.Get(g.specBytes, parts...)
		if err != nil {
			// TODO: better error handling
			log.Fatal(err)
		}

		if refDesc, _ := jp.GetString(value, "description"); refDesc != "" {
			jc.Comment(fmt.Sprintf("%s %s\n", name, refDesc))
		}

		_, _, _, err = jp.Get(value, "enum")
		if err == nil {
			// this is an enum type
			enumType, _ := jp.GetString(value, "type")
			if enumType != "string" && enumType != "object" {
				log.Fatal(fmt.Errorf("unsupported enum type %q", enumType))
			}

			jc.Type().Id(name).String()

			_, err = jp.ArrayEach(value, func(value []byte, dataType jp.ValueType, offset int, err error) {
				enumValue := string(value)
				fn := toGoNameUpper(enumValue)
				jc.Func().Id(name + "_" + fn).Params().Params(jen.Id(name)).Block(jen.Return(jen.Lit(enumValue)))
				//jc.Func().Params(jen.Id(name)).Id(fn).Params().Id(name).Block(jen.Return(jen.Lit(enumValue)))
			}, "enum")
			if err != nil {
				log.Fatal(err)
			}

			writeOutputFile()
			g.refGroup.Done()
			continue
		}

		var structCode []jen.Code

		// one simple trick
		var newValue []byte
		// error is ignored here as allOf may not be present
		_, _ = jp.ArrayEach(value, func(value []byte, dataType jp.ValueType, offset int, err error) {
			refVal, _ := jp.GetString(value, "$ref")
			if refVal != "" {
				structCode = append(structCode, g.qualify(&jen.Statement{}, refVal))
			} else {
				newValue = value
			}
		}, "allOf")

		if newValue != nil {
			value = newValue
		}

		refType, _ := jp.GetString(value, "type")
		baseType, _ := jp.GetBoolean(value, "x-baseType")
		if baseType {
			refType = "baseType"
		}
		switch refType {
		case "baseType":
			// no properties
		case "object":
			// each property
			var propCode []jen.Code
			// if we have properties they must work
			if _, _, _, err = jp.Get(value, "properties"); err == nil {
				if propCode, err = g.propertiesCode(value, ref); err != nil {
					log.Fatalf("properties found but failed to return nil err : %s", err)
				}
				structCode = append(structCode, propCode...)
			} else {
				// properties not found try for example
				if _, _, _, err = jp.Get(value, "example"); err == nil {
					structCode = append(structCode, jen.Comment("TODO: support serialization of examples"))
				} else {
					structCode = append(structCode, jen.Comment("TODO: support serialization of empty object types"))
				}
			}
		}

		jc.Type().Id(name).Struct(structCode...)
		jc.Comment(fmtJson(value))

		writeOutputFile()
		g.refGroup.Done()
	}
}

func (g *Generator) propertiesCode(value []byte, ref string) (propCode []jen.Code, err error) {
	propCode = []jen.Code{}
	err = jp.ObjectEach(value, func(key []byte, value []byte, dataType jp.ValueType, offset int) error {
		propName := string(key)
		propGoName := toGoNameUpper(propName)
		propRef, _ := jp.GetString(value, "$ref")
		propType, _ := jp.GetString(value, "type")
		propFormat, _ := jp.GetString(value, "format")
		propDesc, _ := jp.GetString(value, "description")

		_, _, _, additionalPropertiesErr := jp.Get(value, "additionalProperties")
		if additionalPropertiesErr == nil {
			propType = "additionalProperties"
		}

		if propRef != "" {
			propType = "ref"
		}

		if propDesc != "" {
			propCode = append(propCode, jen.Comment(propDesc))
		}

		field := jen.Id(propGoName)

		switch propType {
		case "string":
			if propFormat == "date-time" {
				field.Qual("time", "Time")
			} else {
				field.String()
			}
		case "boolean":
			field.Bool()
		case "integer", "number":
			field.Int()
		case "ref":
			g.qualify(field.Op("*"), propRef)
		case "array":
			propType, _ = jp.GetString(value, "items", "type")
			if propType == "" {
				propRef, _ = jp.GetString(value, "items", "$ref")
				g.qualify(field.Op("[]*"), propRef)
			} else {
				switch propType {
				case "string":
					field.Op("[]").String()
				case "number":
					field.Op("[]").Int()
				case "object":
					var itemsPropRef string
					if itemsPropRef, err = jp.GetString(value, "items", "additionalProperties", "$ref"); err == nil {
						g.qualify(field.Map(jen.String()), itemsPropRef)
						break
					}
					fmt.Println(fmtJson(value))
					panic(fmt.Errorf("unhandled prop object type %s", propType))
				default:
					fmt.Println(fmtJson(value))
					panic(fmt.Errorf("unhandled prop array type %s", propType))
				}
			}
		case "additionalProperties":
			var mapValueType string
			if mapValueType, err = jp.GetString(value, "additionalProperties", "type"); err == nil {
				switch mapValueType {
				case "string":
					field.Map(jen.String()).String()
				case "object":
					var addPropsProps []byte
					if addPropsProps, _, _, err = jp.Get(value, "additionalProperties", "properties"); err == nil {
						if "{}" == strings.TrimSpace(string(addPropsProps)) {
							field.Map(jen.String()).Any()
						} else {
							fmt.Println(string(addPropsProps))
							fmt.Println(ref)
							fmt.Println(fmtJson(value))
							return fmt.Errorf("additionalProperties case object/properties: fix additionalProperties/type missing case %s", mapValueType)
						}

					} else {
						fmt.Println(ref)
						fmt.Println(fmtJson(value))
						return fmt.Errorf("additionalProperties case object: fix additionalProperties/type missing case %s", mapValueType)
					}
				default:
					fmt.Println(ref)
					fmt.Println(fmtJson(value))
					return fmt.Errorf("additionalProperties case: fix additionalProperties/type missing case %s", mapValueType)
				}
			} else {
				var mapValueRefType string
				mapValueRefType, err = jp.GetString(value, "additionalProperties", "$ref")
				if err == nil {
					g.qualify(field.Map(jen.String()), mapValueRefType)
					break
				}
				fmt.Println(fmtJson(value))
				return fmt.Errorf("additionalProperties case: fix propType %s with %s", propType, ref)
			}
		case "object":
			var addPropsProps []byte
			if addPropsProps, _, _, err = jp.Get(value, "properties"); err == nil {
				if "{}" == strings.TrimSpace(string(addPropsProps)) {
					field.Map(jen.String()).Any()
				} else {
					fmt.Println(string(addPropsProps))
					fmt.Println(ref)
					fmt.Println(fmtJson(value))
					return fmt.Errorf("object case properties: fix %q", string(addPropsProps))
				}

			} else {
				fmt.Println(fmtJson(value))
				return fmt.Errorf("object case: fix propType %s with %s", propType, ref)

			}

		default:
			fmt.Println(fmtJson(value))
			return fmt.Errorf("default case: fix propType %s with %s", propType, ref)
		}
		propCode = append(propCode, field.Tag(map[string]string{"json": propName + ",omitempty"}))
		return nil
	}, "properties")
	return
}

func (g *Generator) Execute() (err error) {

	go g.manager()

	for i := 0; i < 5; i++ {
		go g.refWorker()
	}

	var versions []string
	if versions, err = g.uniqueVersions(); err != nil {
		return
	}

	for _, version := range versions {
		if err = g.createClientFile(version); err != nil {
			return
		}
	}

	err = jp.ObjectEach(g.specBytes, g.processPaths, "paths")

	g.refGroup.Wait()
	close(g.refChan)

	return
}

func (g *Generator) versionDirectory(version string) string {
	return filepath.Join(g.Options.ServiceName + version)
}

func (g *Generator) buildOperation(op *Operation) error {

	// TODO: handle non versioned paths
	version := strings.Split(strings.TrimPrefix(op.Path, "/"), "/")[0]
	packageName := filepath.Join(g.Options.ModuleName, g.Options.ServiceName+version, "client")
	j := jen.NewFilePath(packageName)

	goName := toGoNameUpper(op.XOperationName)

	var doc []string
	doc = append(doc, fmt.Sprintf("%s %s", goName, op.Description))
	for _, p := range op.Parameters {
		// TODO: better doc formatting - like splitting long lines
		doc = append(doc, fmt.Sprintf(" %s - %s", p.Name, p.Description))
	}

	j.Comment(strings.Join(doc, "\n"))

	// TODO: refactor out to method on Parameter ?
	var signature []jen.Code
	for _, p := range op.Parameters {
		var param *jen.Statement
		switch p.In {
		case "query", "header", "path":
			switch p.Type {
			case "string":
				param = jen.Id(p.Name).String()
			case "number":
				param = jen.Id(p.Name).Int()
			case "array":
				switch p.Items {
				case "string":
					param = jen.Id(p.Name).Op("[]").String()
				case "number":
					param = jen.Id(p.Name).Op("[]").Int()
				default:
					if p.ItemsRef != "" {
						param = g.qualify(jen.Id(p.Name).Op("[]"), p.ItemsRef)
						break
					}

					fmt.Println(op.Path)
					panic(fmt.Errorf("unhandled parameter name=%s in=%s type=%s items=%s", p.Name, p.In, p.Type, p.Items))
				}

			}
		case "body":
			im, _ := g.refPathAndType(p.Ref)
			j.ImportAlias(im, filepath.Base(im)+"_")
			param = g.qualify(jen.Id(p.Name).Op("*"), p.Ref)
		}
		if param != nil {
			signature = append(signature, param)
			continue
		}
		panic(fmt.Errorf("unhandled parameter type in=%s type=%s", p.In, p.Type))
	}

	// TODO: handle different payloads for error body
	var result []jen.Code
	hasResponse := op.Responses[0].Ref != ""
	if hasResponse {
		im, _ := g.refPathAndType(op.Responses[0].Ref)
		j.ImportAlias(im, filepath.Base(im)+"_")

		response := jen.Id("response").Op("*")
		g.qualify(response, op.Responses[0].Ref)
		result = append(result, response)
	}
	result = append(result, jen.Err().Error())

	var block []jen.Code
	block = append(block,
		jen.Id("h").Op(":=").Qual("github.com/mlctrez/swaggerlt", "NewRequestHelper").
			Params(jen.Lit(op.Verb), jen.Id("s.Endpoint"), jen.Lit(op.Path)))

	for _, p := range op.Parameters {
		var st *jen.Statement
		switch p.In {
		case "query":
			st = jen.Id("h").Dot("Param").Call(jen.Lit(p.Name), jen.Id(p.Name))
		case "path":
			st = jen.Id("h").Dot("Path").Call(jen.Lit(p.Name), jen.Id(p.Name))
		case "header":
			st = jen.Id("h").Dot("Header").Call(jen.Lit(p.NameOrig), jen.Id(p.Name))
		case "body":
			st = jen.Id("h").Dot("Body").Op("=").Id(p.Name)
		}
		block = append(block, st)
	}

	if hasResponse {
		block = append(block, g.qualify(jen.Id("response").Op("=").Op("&"), op.Responses[0].Ref).Op("{}"))
		//fmt.Fprintf(c, "%s = &%s{}\n", resultVar, resultTypeWithPkg)
		block = append(block, jen.Id("h").Dot("Response").Op("=").Id("response"))
		//fmt.Fprintf(c, "h.Response = %s\n", resultVar)
	}

	for i, res := range op.Responses {
		if i == 0 || res.Code == 0 {
			continue
		}
		if res.Ref != "" {
			st := jen.Id("h").Dot("ResponseType").
				Call(jen.Lit(res.Code), g.qualify((&jen.Statement{}).Op("&"), res.Ref).Op("{}"))
			block = append(block, st)
		}
	}

	block = append(block, jen.Err().Op("=").Id("h").Dot("Execute").
		Call(jen.Id("s").Dot("Client")))
	//fmt.Fprintf(c, "err = h.Execute(s.Client)\n")

	block = append(block, jen.Return())

	receiver := jen.Id("s").Op("*").Id("Client")
	j.Func().Params(receiver).Id(goName).Params(signature...).Params(result...).Block(block...)

	j.Comment(fmtJson(op.RawData))

	// write out the file
	goFile := fmt.Sprintf("%s.go", toGoNameLower(op.XOperationName))
	path := filepath.Join(g.Options.ServiceName+version, "client", goFile)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if create, err := os.Create(path); err != nil {
		return err
	} else {
		defer func() { _ = create.Close() }()
		return j.Render(create)
	}

}

func (g *Generator) processPaths(pathBytes []byte, value []byte, _ jp.ValueType, _ int) error {

	path := string(pathBytes)
	regex := g.Options.PathRegex

	if regex.MatchString(path) {
		return jp.ObjectEach(value, func(verbBytes []byte, value []byte, _ jp.ValueType, _ int) (err error) {

			verb := string(verbBytes)
			var op *Operation

			if op, err = OperationFromSpec(path, verb, value); err != nil {
				return
			}

			return g.buildOperation(op)
		})
	}
	return nil
}

func (g *Generator) refPathAndType(ref string) (string, string) {

	if ref == "" {
		panic("refPathAndType: ref was empty string")
	}

	refParts := strings.Split(ref, "/")
	options := g.Options

	// last part - v0.developmentEvents.subscriber.CreateSubscriberRequest
	refName := refParts[len(refParts)-1]
	refNameParts := strings.Split(refName, ".")

	pathParts := []string{options.ModuleName}
	pathParts = append(pathParts, options.ServiceName+refNameParts[0])
	if len(refNameParts) > 1 {
		pathParts = append(pathParts, refNameParts[1:len(refNameParts)-1]...)
	}

	path := filepath.Join(pathParts...)
	if strings.HasSuffix(path, "/type") {
		// this is what the jennifer codebase does for conflicting go keywords
		path += "1"
	}

	refType := refNameParts[len(refNameParts)-1]
	refType = toGoNameUpper(refType)
	return path, refType
}

func (g *Generator) qualify(s *jen.Statement, ref string) *jen.Statement {

	path, refType := g.refPathAndType(ref)
	g.refGroup.Add(1)
	g.refManager <- ref
	s.Qual(path, refType)

	return s
}

type Parameter struct {
	Name        string `json:"name"`
	NameOrig    string `json:"name_orig"`
	In          string `json:"in"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Type        string `json:"type,omitempty"`
	Ref         string `json:"ref,omitempty"`
	Items       string `json:"items,omitempty"`
	ItemsRef    string `json:"itemsRef,omitempty"`
}

func fmtJson(value []byte) string {
	m := make(map[string]any)
	err := json.Unmarshal(value, &m)
	if err != nil {
		return err.Error()
	}
	indent, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err.Error()
	}
	return string(indent)

}

type Response struct {
	Code        int    `json:"code"`
	Description string `json:"description,omitempty"`
	Ref         string `json:"ref,omitempty"`
}

func (g *Generator) uniqueVersions() (result []string, err error) {
	err = jp.ObjectEach(g.specBytes, func(key []byte, _ []byte, _ jp.ValueType, _ int) error {
		path := string(key)
		if g.Options.PathRegex.MatchString(path) {
			result = append(result, strings.Split(path, "/")[1])
		}
		return nil
	}, "paths")
	return
}
