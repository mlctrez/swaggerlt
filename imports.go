package swaggerlt

import "fmt"

type Imports []string

func (i *Imports) Add(path string) {
	if path == "" {
		return
	}
	existing := false
	for _, s := range *i {
		if s == path {
			existing = true
		}
	}
	if !existing {
		*i = append(*i, path)
	}
}

func (i *Imports) Code() string {
	if len(*i) == 0 {
		return ""
	}
	out := "import (\n"
	for _, s := range *i {
		out += fmt.Sprintf("\t%q\n", s)
	}
	out += ")\n"
	return out
}

func (i *Imports) Append(imports *Imports) {
	for _, s := range *imports {
		i.Add(s)
	}
}
