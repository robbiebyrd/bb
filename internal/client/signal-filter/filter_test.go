package signalfilter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	signalfilter "github.com/robbiebyrd/cantou/internal/client/signal-filter"
	canModels "github.com/robbiebyrd/cantou/internal/models"
)

func sig(message, signal string) canModels.CanSignalTimestamped {
	return canModels.CanSignalTimestamped{Message: message, Signal: signal}
}

// --- individual rule matching ---

func TestRule_SignalEq(t *testing.T) {
	r := signalfilter.Rule{Field: signalfilter.FieldSignal, Op: signalfilter.OpEq, Value: "RPM"}
	assert.True(t, r.Match(sig("", "RPM")))
	assert.False(t, r.Match(sig("", "Speed")))
}

func TestRule_SignalNotEq(t *testing.T) {
	r := signalfilter.Rule{Field: signalfilter.FieldSignal, Op: signalfilter.OpNotEq, Value: "RPM"}
	assert.False(t, r.Match(sig("", "RPM")))
	assert.True(t, r.Match(sig("", "Speed")))
}

func TestRule_SignalContains(t *testing.T) {
	r := signalfilter.Rule{Field: signalfilter.FieldSignal, Op: signalfilter.OpContains, Value: "UNUSED"}
	assert.True(t, r.Match(sig("", "S01_UNUSED_01")))
	assert.False(t, r.Match(sig("", "RPM")))
}

func TestRule_SignalNotContains(t *testing.T) {
	r := signalfilter.Rule{Field: signalfilter.FieldSignal, Op: signalfilter.OpNotContains, Value: "UNUSED"}
	assert.False(t, r.Match(sig("", "S01_UNUSED_01")))
	assert.True(t, r.Match(sig("", "RPM")))
}

func TestRule_SignalStartsWith(t *testing.T) {
	r := signalfilter.Rule{Field: signalfilter.FieldSignal, Op: signalfilter.OpStartsWith, Value: "S01"}
	assert.True(t, r.Match(sig("", "S01_RPM")))
	assert.False(t, r.Match(sig("", "RPM")))
}

func TestRule_SignalNotStartsWith(t *testing.T) {
	r := signalfilter.Rule{Field: signalfilter.FieldSignal, Op: signalfilter.OpNotStartsWith, Value: "S01"}
	assert.False(t, r.Match(sig("", "S01_RPM")))
	assert.True(t, r.Match(sig("", "RPM")))
}

func TestRule_SignalEndsWith(t *testing.T) {
	r := signalfilter.Rule{Field: signalfilter.FieldSignal, Op: signalfilter.OpEndsWith, Value: "_raw"}
	assert.True(t, r.Match(sig("", "speed_raw")))
	assert.False(t, r.Match(sig("", "speed")))
}

func TestRule_SignalNotEndsWith(t *testing.T) {
	r := signalfilter.Rule{Field: signalfilter.FieldSignal, Op: signalfilter.OpNotEndsWith, Value: "_raw"}
	assert.False(t, r.Match(sig("", "speed_raw")))
	assert.True(t, r.Match(sig("", "speed")))
}

func TestRule_MessageField(t *testing.T) {
	r := signalfilter.Rule{Field: signalfilter.FieldMessage, Op: signalfilter.OpStartsWith, Value: "UNKNOWN_"}
	assert.True(t, r.Match(sig("UNKNOWN_0x1A2", "")))
	assert.False(t, r.Match(sig("EngineData", "")))
}

// --- group: exclude mode (default) ---

func TestGroup_ExcludeAnd_DropsWhenAllRulesMatch(t *testing.T) {
	g := signalfilter.Group{
		Rules: []signalfilter.Rule{
			{Field: signalfilter.FieldSignal, Op: signalfilter.OpContains, Value: "UNUSED"},
			{Field: signalfilter.FieldMessage, Op: signalfilter.OpStartsWith, Value: "UNKNOWN_"},
		},
		Op:   signalfilter.GroupOpAnd,
		Mode: signalfilter.ModeExclude,
	}
	assert.False(t, g.Allow(sig("UNKNOWN_0x1A2", "S01_UNUSED_01"))) // both match → drop
	assert.True(t, g.Allow(sig("EngineData", "S01_UNUSED_01")))     // only signal matches → keep
	assert.True(t, g.Allow(sig("UNKNOWN_0x1A2", "RPM")))            // only message matches → keep
	assert.True(t, g.Allow(sig("EngineData", "RPM")))               // neither matches → keep
}

func TestGroup_ExcludeOr_DropsWhenAnyRuleMatches(t *testing.T) {
	g := signalfilter.Group{
		Rules: []signalfilter.Rule{
			{Field: signalfilter.FieldSignal, Op: signalfilter.OpContains, Value: "UNUSED"},
			{Field: signalfilter.FieldMessage, Op: signalfilter.OpStartsWith, Value: "UNKNOWN_"},
		},
		Op:   signalfilter.GroupOpOr,
		Mode: signalfilter.ModeExclude,
	}
	assert.False(t, g.Allow(sig("UNKNOWN_0x1A2", "S01_UNUSED_01"))) // both match → drop
	assert.False(t, g.Allow(sig("EngineData", "S01_UNUSED_01")))    // signal matches → drop
	assert.False(t, g.Allow(sig("UNKNOWN_0x1A2", "RPM")))           // message matches → drop
	assert.True(t, g.Allow(sig("EngineData", "RPM")))               // neither → keep
}

// --- group: include mode ---

func TestGroup_IncludeAnd_KeepsOnlyWhenAllRulesMatch(t *testing.T) {
	g := signalfilter.Group{
		Rules: []signalfilter.Rule{
			{Field: signalfilter.FieldSignal, Op: signalfilter.OpStartsWith, Value: "S01"},
			{Field: signalfilter.FieldMessage, Op: signalfilter.OpEq, Value: "OBD2"},
		},
		Op:   signalfilter.GroupOpAnd,
		Mode: signalfilter.ModeInclude,
	}
	assert.True(t, g.Allow(sig("OBD2", "S01_RPM")))   // both match → keep
	assert.False(t, g.Allow(sig("OBD2", "RPM")))      // only message matches → drop
	assert.False(t, g.Allow(sig("Other", "S01_RPM"))) // only signal matches → drop
	assert.False(t, g.Allow(sig("Other", "RPM")))     // neither → drop
}

func TestGroup_IncludeOr_KeepsWhenAnyRuleMatches(t *testing.T) {
	g := signalfilter.Group{
		Rules: []signalfilter.Rule{
			{Field: signalfilter.FieldSignal, Op: signalfilter.OpEq, Value: "RPM"},
			{Field: signalfilter.FieldMessage, Op: signalfilter.OpEq, Value: "OBD2"},
		},
		Op:   signalfilter.GroupOpOr,
		Mode: signalfilter.ModeInclude,
	}
	assert.True(t, g.Allow(sig("OBD2", "RPM")))    // both → keep
	assert.True(t, g.Allow(sig("Other", "RPM")))   // signal matches → keep
	assert.True(t, g.Allow(sig("OBD2", "Speed")))  // message matches → keep
	assert.False(t, g.Allow(sig("Other", "Speed"))) // neither → drop
}

// --- empty group always allows ---

func TestGroup_Empty_AlwaysAllows(t *testing.T) {
	g := signalfilter.Group{Mode: signalfilter.ModeExclude, Op: signalfilter.GroupOpAnd}
	assert.True(t, g.Allow(sig("anything", "anything")))
}

// --- parsing ---

func TestParseRule_Valid(t *testing.T) {
	cases := []struct {
		input string
		field signalfilter.Field
		op    signalfilter.Op
		value string
	}{
		{"signal:eq:RPM", signalfilter.FieldSignal, signalfilter.OpEq, "RPM"},
		{"message:startswith:UNKNOWN_", signalfilter.FieldMessage, signalfilter.OpStartsWith, "UNKNOWN_"},
		{"signal:notcontains:UNUSED", signalfilter.FieldSignal, signalfilter.OpNotContains, "UNUSED"},
		{"signal:endswith:_raw", signalfilter.FieldSignal, signalfilter.OpEndsWith, "_raw"},
		{"signal:notendswith:_raw", signalfilter.FieldSignal, signalfilter.OpNotEndsWith, "_raw"},
		{"message:notstartswith:UNK", signalfilter.FieldMessage, signalfilter.OpNotStartsWith, "UNK"},
		{"signal:neq:PID", signalfilter.FieldSignal, signalfilter.OpNotEq, "PID"},
		{"signal:contains:foo", signalfilter.FieldSignal, signalfilter.OpContains, "foo"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			r, err := signalfilter.ParseRule(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.field, r.Field)
			assert.Equal(t, tc.op, r.Op)
			assert.Equal(t, tc.value, r.Value)
		})
	}
}

func TestParseRule_InvalidFormat(t *testing.T) {
	_, err := signalfilter.ParseRule("signal:eq")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "field:op:value")
}

func TestParseRule_UnknownField(t *testing.T) {
	_, err := signalfilter.ParseRule("data:eq:foo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "field")
}

func TestParseRule_UnknownOp(t *testing.T) {
	_, err := signalfilter.ParseRule("signal:regex:foo")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "op")
}

func TestParseGroup_Valid(t *testing.T) {
	rules := []string{"signal:eq:RPM", "message:startswith:UNKNOWN_"}
	g, err := signalfilter.ParseGroup(rules, "or", "exclude")
	assert.NoError(t, err)
	assert.Equal(t, signalfilter.GroupOpOr, g.Op)
	assert.Equal(t, signalfilter.ModeExclude, g.Mode)
	assert.Len(t, g.Rules, 2)
}

func TestParseGroup_InvalidOp(t *testing.T) {
	_, err := signalfilter.ParseGroup([]string{"signal:eq:RPM"}, "xor", "exclude")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "op")
}

func TestParseGroup_InvalidMode(t *testing.T) {
	_, err := signalfilter.ParseGroup([]string{"signal:eq:RPM"}, "and", "maybe")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mode")
}

func TestParseGroup_EmptyRules(t *testing.T) {
	g, err := signalfilter.ParseGroup(nil, "and", "exclude")
	assert.NoError(t, err)
	assert.Empty(t, g.Rules)
}
