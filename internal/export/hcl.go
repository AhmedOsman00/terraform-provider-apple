// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package export

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// file is one generated .tf file, built up block by block.
type file struct {
	name string
	hcl  *hclwrite.File
}

func newFile(name string) *file {
	return &file{name: name, hcl: hclwrite.NewEmptyFile()}
}

// Bytes is the file's content, formatted the way terraform fmt would.
func (f *file) Bytes() []byte {
	return hclwrite.Format(f.hcl.Bytes())
}

func (f *file) comment(text string) {
	body := f.hcl.Body()
	for _, line := range strings.Split(text, "\n") {
		body.AppendUnstructuredTokens(hclwrite.Tokens{{
			Type:  hclsyntax.TokenComment,
			Bytes: []byte(strings.TrimRight("# "+line, " ") + "\n"),
		}})
	}
	body.AppendNewline()
}

// address is a resource or data source address that other blocks can refer to.
type address struct {
	data     bool
	typeName string
	label    string
}

func (a address) String() string {
	if a.data {
		return "data." + a.typeName + "." + a.label
	}
	return a.typeName + "." + a.label
}

func (a address) traversal(attributes ...any) hcl.Traversal {
	var traversal hcl.Traversal
	if a.data {
		traversal = hcl.Traversal{hcl.TraverseRoot{Name: "data"}, hcl.TraverseAttr{Name: a.typeName}}
	} else {
		traversal = hcl.Traversal{hcl.TraverseRoot{Name: a.typeName}}
	}
	traversal = append(traversal, hcl.TraverseAttr{Name: a.label})
	for _, step := range attributes {
		switch s := step.(type) {
		case string:
			traversal = append(traversal, hcl.TraverseAttr{Name: s})
		case int:
			traversal = append(traversal, hcl.TraverseIndex{Key: cty.NumberIntVal(int64(s))})
		}
	}
	return traversal
}

// value is what an attribute is set to: a literal, or a reference to another
// exported block. A reference is what wires the dependency Terraform needs
// between, say, a profile and the Bundle ID it is for.
type value struct {
	literal   *cty.Value
	reference hcl.Traversal
}

func literal(v cty.Value) value { return value{literal: &v} }

func ref(traversal hcl.Traversal) value { return value{reference: traversal} }

func (v value) tokens() hclwrite.Tokens {
	if v.reference != nil {
		return hclwrite.TokensForTraversal(v.reference)
	}
	return hclwrite.TokensForValue(*v.literal)
}

// block is a resource or data source being written. Attributes are written in
// the order they are set, and unset optional attributes are left out, which is
// the same as null to Terraform.
type block struct {
	body *hclwrite.Body
}

func (b *block) set(name string, v value) {
	b.body.SetAttributeRaw(name, v.tokens())
}

func (b *block) str(name, s string) { b.set(name, literal(cty.StringVal(s))) }

// optStr sets an attribute only when Apple reported a value for it.
func (b *block) optStr(name string, s *string) {
	if s != nil {
		b.str(name, *s)
	}
}

// optBool sets an attribute only when Apple reported a value for it.
func (b *block) optBool(name string, v *bool) {
	if v != nil {
		b.set(name, literal(cty.BoolVal(*v)))
	}
}

func (b *block) optInt(name string, v *int64) {
	if v != nil {
		b.set(name, literal(cty.NumberIntVal(*v)))
	}
}

// strings writes an ordered list of strings.
func (b *block) strings(name string, values []string) {
	elements := make([]hclwrite.Tokens, 0, len(values))
	for _, v := range values {
		elements = append(elements, hclwrite.TokensForValue(cty.StringVal(v)))
	}
	b.body.SetAttributeRaw(name, tupleTokens(elements))
}

// refs writes a list whose elements are each a literal or a reference, in order.
func (b *block) refs(name string, values []value) {
	elements := make([]hclwrite.Tokens, 0, len(values))
	for _, v := range values {
		elements = append(elements, v.tokens())
	}
	b.body.SetAttributeRaw(name, tupleTokens(elements))
}

// objects writes a list of objects, leaving out each object's null attributes.
// It serves both list and set attributes: Terraform converts a tuple to either.
func (b *block) objects(name string, objects []map[string]cty.Value) {
	elements := make([]hclwrite.Tokens, 0, len(objects))
	for _, object := range objects {
		keys := make([]string, 0, len(object))
		for key, v := range object {
			if !v.IsNull() {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)

		attributes := make([]hclwrite.ObjectAttrTokens, 0, len(keys))
		for _, key := range keys {
			attributes = append(attributes, hclwrite.ObjectAttrTokens{
				Name:  hclwrite.TokensForIdentifier(key),
				Value: hclwrite.TokensForValue(object[key]),
			})
		}
		elements = append(elements, hclwrite.TokensForObject(attributes))
	}
	b.body.SetAttributeRaw(name, tupleTokens(elements))
}

// tupleTokens writes a list on one line when it is short, and one element per
// line otherwise, which is what terraform fmt leaves alone and what a diff
// of a long territory list can be read in.
func tupleTokens(elements []hclwrite.Tokens) hclwrite.Tokens {
	multiline := len(elements) > 3
	for _, element := range elements {
		for _, token := range element {
			if token.Type == hclsyntax.TokenNewline {
				multiline = true
			}
		}
	}
	if !multiline {
		return hclwrite.TokensForTuple(elements)
	}

	tokens := hclwrite.Tokens{
		{Type: hclsyntax.TokenOBrack, Bytes: []byte("[")},
		{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
	}
	for _, element := range elements {
		tokens = append(tokens, element...)
		tokens = append(tokens,
			&hclwrite.Token{Type: hclsyntax.TokenComma, Bytes: []byte(",")},
			&hclwrite.Token{Type: hclsyntax.TokenNewline, Bytes: []byte("\n")},
		)
	}
	return append(tokens, &hclwrite.Token{Type: hclsyntax.TokenCBrack, Bytes: []byte("]")})
}

// optString is a cty string, or null when Apple reported none.
func optString(s *string) cty.Value {
	if s == nil {
		return cty.NullVal(cty.String)
	}
	return cty.StringVal(*s)
}

func optBoolValue(v *bool) cty.Value {
	if v == nil {
		return cty.NullVal(cty.Bool)
	}
	return cty.BoolVal(*v)
}

// labels hands out Terraform labels that are valid, readable and unique per
// type: com.example.app becomes com_example_app, and a second one
// com_example_app_2.
type labels struct {
	used map[string]bool
}

var labelInvalid = regexp.MustCompile(`[^a-z0-9_]+`)

func slug(text string) string {
	label := strings.Trim(labelInvalid.ReplaceAllString(strings.ToLower(text), "_"), "_")
	if label == "" {
		return "unnamed"
	}
	if label[0] >= '0' && label[0] <= '9' {
		return "r_" + label
	}
	return label
}

func (l *labels) next(typeName string, parts ...string) string {
	if l.used == nil {
		l.used = map[string]bool{}
	}

	var named []string
	for _, part := range parts {
		if part != "" {
			named = append(named, part)
		}
	}
	base := slug(strings.Join(named, "_"))

	label := base
	for i := 2; l.used[typeName+"."+label]; i++ {
		label = fmt.Sprintf("%s_%d", base, i)
	}
	l.used[typeName+"."+label] = true
	return label
}

func ctyString(s string) cty.Value { return cty.StringVal(s) }

// sortedKeys returns a map's keys in order, so output never depends on Go's
// map iteration order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// tupleOfTraversals writes a list of bare references, as depends_on takes.
func tupleOfTraversals(traversals ...hcl.Traversal) hclwrite.Tokens {
	elements := make([]hclwrite.Tokens, 0, len(traversals))
	for _, traversal := range traversals {
		elements = append(elements, hclwrite.TokensForTraversal(traversal))
	}
	return hclwrite.TokensForTuple(elements)
}
