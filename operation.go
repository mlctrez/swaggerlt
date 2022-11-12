package swaggerlt

import (
	jp "github.com/buger/jsonparser"
	"strconv"
)

type Operation struct {
	Path           string       `json:"path"`
	Verb           string       `json:"verb"`
	Tags           []string     `json:"tags"`
	Description    string       `json:"description"`
	Parameters     []*Parameter `json:"parameters,omitempty"`
	Responses      []*Response  `json:"responses,omitempty"`
	XOperationName string       `json:"x-operation-name"`
	RawData        []byte       `json:"-"`
}

func OperationFromSpec(path, verb string, value []byte) (op *Operation, err error) {
	op = &Operation{Path: path, Verb: verb}
	if _, err = jp.ArrayEach(value, op.tags, "tags"); err != nil {
		return
	}
	op.Description, _ = jp.GetString(value, "description")
	if _, err = jp.ArrayEach(value, op.parameters, "parameters"); err != nil {
		return
	}
	if err = jp.ObjectEach(value, op.responses, "responses"); err != nil {
		return
	}

	op.XOperationName, _ = jp.GetString(value, "x-operation-name")
	op.RawData = value

	return
}

func (op *Operation) tags(value []byte, _ jp.ValueType, _ int, _ error) {
	op.Tags = append(op.Tags, string(value))
}

func (op *Operation) parameters(value []byte, _ jp.ValueType, _ int, _ error) {
	p := &Parameter{}
	p.Name, _ = jp.GetString(value, "name")
	p.NameOrig, _ = jp.GetString(value, "name")
	p.Name = toGoNameLower(p.Name)

	p.In, _ = jp.GetString(value, "in")
	p.Description, _ = jp.GetString(value, "description")
	p.Required, _ = jp.GetBoolean(value, "required")
	p.Type, _ = jp.GetString(value, "type")
	p.Items, _ = jp.GetString(value, "items", "type")

	p.Ref, _ = jp.GetString(value, "schema", "$ref")

	// TODO minimum, maximum, multipleOf
	op.Parameters = append(op.Parameters, p)
}

func (op *Operation) responses(key []byte, value []byte, _ jp.ValueType, _ int) error {

	code, err := strconv.Atoi(string(key))
	if string(key) == "default" {
		code = 0
		err = nil
	}
	if err != nil {
		return err
	}
	r := &Response{Code: code}
	r.Description, _ = jp.GetString(value, "description")
	r.Ref, _ = jp.GetString(value, "schema", "$ref")

	op.Responses = append(op.Responses, r)
	return nil
}
