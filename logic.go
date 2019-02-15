//Package logic provides primitives for boolean evaluation which checks if a
//specific token (string) exists (or not) in the given feature set (string slice).
package logic

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

//Expression represents a logic operation, which can be: "all_of" ("and"), "any_of" ("or"),
//"none_of" ("not") or "n of", where n is a non-negative integer. If n equals 0, it is same
//as "none_of"; if n is 1, same as "any_of" or "or"; if n equal to the number of items,
//means "all_of", or "and"; if n is larger than the number of items, the expression will
//always evaluate to false.
//
//The operands of a logic operation must be a slice whose elements could be either a
//feature (string) or a (sub)Expression.
//
//If a feature string starts with tilde (~), its a regular expression, otherwise, a raw
//string (which is case sensitive).
type Expression struct {
	verb string
	rate int
	subj []interface{}
}

//UnmarshalYAML implements the yaml unmarshal interface
func (x *Expression) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var ms map[interface{}]interface{}
	err := unmarshal(&ms)
	if err != nil {
		return err
	}
	p, err := load(ms)
	*x = *p
	return err
}

//MarshalYAML implements the yaml marshal interface
func (x Expression) MarshalYAML() (interface{}, error) {
	return x.export(), nil
}

func (x Expression) export() map[string]interface{} {
	var subj []interface{}
	for _, s := range x.subj {
		switch s.(type) {
		case string:
			subj = append(subj, s)
		case *Expression:
			e := s.(*Expression).export()
			if e == nil {
				return nil
			}
			subj = append(subj, e)
		default:
			return nil
		}
	}
	return map[string]interface{}{x.verb: subj}
}

//Save writes the logic expression as YAML string to the given writer.
func (x Expression) Save(w io.Writer) error {
	return yaml.NewEncoder(w).Encode(x.export())
}

//String output the logic expression as YAML string.
func (x Expression) String() string {
	var buf bytes.Buffer
	x.Save(&buf)
	return buf.String()
}

func load(ms map[interface{}]interface{}) (*Expression, error) {
	if len(ms) != 1 {
		return nil, fmt.Errorf("expect 1 verb, got %d", len(ms))
	}
	var v string
	var r int
	var s []interface{}
	for verb, subj := range ms {
		v = verb.(string)
		switch v {
		case "not", "none_of":
			v = "none_of"
			r = 0
		case "and", "all_of":
			v = "all_of"
			r = -1
		case "or", "any_of":
			v = "any_of"
			r = 1
		default:
			if strings.HasSuffix(v, "_of") {
				c, err := strconv.Atoi(v[:len(v)-3])
				if err == nil && c >= 0 {
					if c == 0 {
						v = "none_of"
					}
					r = c
					break
				}
			}
			return nil, fmt.Errorf("invalid verb: %v", v)
		}
		js, ok := subj.([]interface{})
		if !ok {
			return nil, fmt.Errorf("subject must be slice")
		}
		for _, j := range js {
			switch j.(type) {
			case string:
				s = append(s, j)
			case map[interface{}]interface{}:
				o, err := load(j.(map[interface{}]interface{}))
				if err != nil {
					return nil, err
				}
				s = append(s, o)
			default:
				return nil, fmt.Errorf("invalid subject item: %v", j)
			}
		}
	}
	return &Expression{verb: v, rate: r, subj: s}, nil
}

//Load read YAML string and parse it as logic expression. If reading from
//the reader fails, or the data is not valid, an error is returned, along
//with nil expression.  Valid YAML is a one-element map whose key must be
//one of the defined logic operators: "all_of" ("and"), "any_of" ("or"),
//"none_of" ("not"), or "n_of" (where n is a non-negative integer); and
//value must be a slice of either string or nested expression. For example:
//
//    ---
//    and:
//    - item1
//    - or: [item2, item3]
//
//The above YAML defines "item1 and (item2 or item3)".  For more examples,
//see the test file.
func Load(r io.Reader) (*Expression, error) {
	var ms map[interface{}]interface{}
	err := yaml.NewDecoder(r).Decode(&ms)
	if err != nil {
		return nil, err
	}
	return load(ms)
}

//Parse parse the given YAML string as logic expression.
func Parse(exp string) (*Expression, error) {
	return Load(bytes.NewBufferString(exp))
}

func eval(token string, featrues []string) bool {
	if strings.HasPrefix(token, "~") {
		rx := regexp.MustCompile(token[1:])
		for _, f := range featrues {
			if rx.MatchString(f) {
				return true
			}
		}
		return false
	}
	for _, f := range featrues {
		if f == token {
			return true
		}
	}
	return false
}

func (x Expression) evalNeg(subj []interface{}, features []string) bool {
	for _, s := range subj {
		var res bool
		switch s.(type) {
		case string:
			res = eval(s.(string), features)
		default:
			res = s.(*Expression).Eval(features)
		}
		if res {
			return false
		}
	}
	return true
}

func (x Expression) evalPos(subj []interface{}, features []string) bool {
	rate := x.rate
	if rate < 0 {
		rate = len(subj)
	}
	hit := 0
	for _, s := range subj {
		var res bool
		switch s.(type) {
		case string:
			res = eval(s.(string), features)
		default:
			res = s.(*Expression).Eval(features)
		}
		if res {
			hit++
		}
		if hit >= rate {
			return true
		}
	}
	return false
}

//Eval evaluate the given feature set against the logic expression.
func (x Expression) Eval(features []string) bool {
	switch x.verb {
	case "none_of":
		return x.evalNeg(x.subj, features)
	default:
		return x.evalPos(x.subj, features)
	}
}
