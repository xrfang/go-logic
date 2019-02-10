//Package yamlogic provides primitives for boolean evaluation which checks if a
//specific token (string) exists (or not) in the given feature set (string slice).
package yamlogic

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

//Expression represents a logic operation, which must be one of "not", "and", "or".
//The operands of a logic operation must be a slice whose elements could be either
//a feature (string) or a (sub)Expression.
//
//If a feature string starts with tilde (~), its a regular expression, otherwise,
//a raw string (which is case sensitive).
type Expression struct {
	verb string
	subj []interface{}
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
	var s []interface{}
	for verb, subj := range ms {
		switch verb {
		case "not", "and", "or":
		default:
			return nil, fmt.Errorf("invalid verb: %v", verb)
		}
		v = verb.(string)
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
	return &Expression{verb: v, subj: s}, nil
}

//Load read YAML string and parse it as logic expression. If reading from
//the reader fails, or the data is not valid, an error is returned, along
//with nil expression.  Valid YAML is a one-element map whose key must be
//"not", "and", "or", and value must be a slice of either string or nested
//expression.   For example:
//
//    ---
//    and:
//    - item1
//    - or: [item2, item3]
//
//The above YAML defines "item1 and (item2 or item3)".
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

func (x Expression) evalNot(subj []interface{}, features []string) bool {
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

func (x Expression) evalAnd(subj []interface{}, features []string) bool {
	for _, s := range subj {
		var res bool
		switch s.(type) {
		case string:
			res = eval(s.(string), features)
		default:
			res = s.(*Expression).Eval(features)
		}
		if !res {
			return false
		}
	}
	return true
}

func (x Expression) evalOr(subj []interface{}, features []string) bool {
	for _, s := range subj {
		var res bool
		switch s.(type) {
		case string:
			res = eval(s.(string), features)
		default:
			res = s.(*Expression).Eval(features)
		}
		if res {
			return true
		}
	}
	return false
}

//Eval evaluate the given feature set against the logic expression.
func (x Expression) Eval(features []string) bool {
	switch x.verb {
	case "not":
		return x.evalNot(x.subj, features)
	case "and":
		return x.evalAnd(x.subj, features)
	default:
		return x.evalOr(x.subj, features)
	}
}
