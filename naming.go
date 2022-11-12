package swaggerlt

import (
	"strings"
)

func toGoNameUpper(in string) string {
	in = toGoName(in)

	return strings.ToUpper(in[0:1]) + in[1:]
}

func toGoNameLower(in string) string {
	in = toGoName(in)
	return strings.ToLower(in[0:1]) + in[1:]
}

func toGoName(in string) string {

	if in == "" {
		panic("toGoName with empty string")
	}

	// remove leading _ as in _links
	for strings.HasPrefix(in, "_") {
		in = strings.TrimPrefix(in, "_")
	}
	// no dots
	in = strings.ReplaceAll(in, ".", "")
	// no dashes
	in = strings.ReplaceAll(in, "-", "_")
	in = strings.ReplaceAll(in, ":", "_")

	if in == "type" {
		return "type_"
	}
	if in[0] >= '0' && in[0] <= '9' {
		return "n" + in
	}

	return in
}
