package signalfilter

import (
	"fmt"
	"strings"

	canModels "github.com/robbiebyrd/cantou/internal/models"
)

type Field int

const (
	FieldSignal Field = iota
	FieldMessage
)

type Op int

const (
	OpEq Op = iota
	OpNotEq
	OpContains
	OpNotContains
	OpStartsWith
	OpNotStartsWith
	OpEndsWith
	OpNotEndsWith
)

type GroupOp int

const (
	GroupOpAnd GroupOp = iota
	GroupOpOr
)

type Mode int

const (
	ModeExclude Mode = iota // signals matching the group are dropped
	ModeInclude             // only signals matching the group are kept
)

// Rule is a single field:op:value predicate against a CanSignalTimestamped.
type Rule struct {
	Field Field
	Op    Op
	Value string
}

// Match reports whether the rule matches sig.
func (r Rule) Match(sig canModels.CanSignalTimestamped) bool {
	var subject string
	switch r.Field {
	case FieldSignal:
		subject = sig.Signal
	case FieldMessage:
		subject = sig.Message
	}
	switch r.Op {
	case OpEq:
		return subject == r.Value
	case OpNotEq:
		return subject != r.Value
	case OpContains:
		return strings.Contains(subject, r.Value)
	case OpNotContains:
		return !strings.Contains(subject, r.Value)
	case OpStartsWith:
		return strings.HasPrefix(subject, r.Value)
	case OpNotStartsWith:
		return !strings.HasPrefix(subject, r.Value)
	case OpEndsWith:
		return strings.HasSuffix(subject, r.Value)
	case OpNotEndsWith:
		return !strings.HasSuffix(subject, r.Value)
	}
	return false
}

// Group is a set of Rules combined with a GroupOp, applied in the given Mode.
type Group struct {
	Rules []Rule
	Op    GroupOp
	Mode  Mode
}

// Allow reports whether sig should be forwarded.
// An empty group always allows.
func (g Group) Allow(sig canModels.CanSignalTimestamped) bool {
	if len(g.Rules) == 0 {
		return true
	}

	matched := false
	switch g.Op {
	case GroupOpOr:
		for _, r := range g.Rules {
			if r.Match(sig) {
				matched = true
				break
			}
		}
	default: // GroupOpAnd
		matched = true
		for _, r := range g.Rules {
			if !r.Match(sig) {
				matched = false
				break
			}
		}
	}

	if g.Mode == ModeInclude {
		return matched
	}
	return !matched // ModeExclude
}

// ParseRule parses a "field:op:value" string into a Rule.
func ParseRule(s string) (Rule, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return Rule{}, fmt.Errorf("signal filter rule %q must be in field:op:value format", s)
	}

	var field Field
	switch strings.ToLower(parts[0]) {
	case "signal":
		field = FieldSignal
	case "message":
		field = FieldMessage
	default:
		return Rule{}, fmt.Errorf("signal filter rule %q: unknown field %q (must be signal or message)", s, parts[0])
	}

	var op Op
	switch strings.ToLower(parts[1]) {
	case "eq":
		op = OpEq
	case "neq":
		op = OpNotEq
	case "contains":
		op = OpContains
	case "notcontains":
		op = OpNotContains
	case "startswith":
		op = OpStartsWith
	case "notstartswith":
		op = OpNotStartsWith
	case "endswith":
		op = OpEndsWith
	case "notendswith":
		op = OpNotEndsWith
	default:
		return Rule{}, fmt.Errorf("signal filter rule %q: unknown op %q (must be eq, neq, contains, notcontains, startswith, notstartswith, endswith, notendswith)", s, parts[1])
	}

	return Rule{Field: field, Op: op, Value: parts[2]}, nil
}

// ParseGroup parses a slice of "field:op:value" strings plus op and mode strings into a Group.
// op must be "and" or "or" (default "and"). mode must be "exclude" or "include" (default "exclude").
func ParseGroup(ruleStrs []string, opStr, modeStr string) (Group, error) {
	var groupOp GroupOp
	switch strings.ToLower(opStr) {
	case "", "and":
		groupOp = GroupOpAnd
	case "or":
		groupOp = GroupOpOr
	default:
		return Group{}, fmt.Errorf("signal filter op %q is invalid (must be and or or)", opStr)
	}

	var mode Mode
	switch strings.ToLower(modeStr) {
	case "", "exclude":
		mode = ModeExclude
	case "include":
		mode = ModeInclude
	default:
		return Group{}, fmt.Errorf("signal filter mode %q is invalid (must be exclude or include)", modeStr)
	}

	rules := make([]Rule, 0, len(ruleStrs))
	for _, s := range ruleStrs {
		r, err := ParseRule(s)
		if err != nil {
			return Group{}, err
		}
		rules = append(rules, r)
	}

	return Group{Rules: rules, Op: groupOp, Mode: mode}, nil
}
