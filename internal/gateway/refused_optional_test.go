package gateway

import "testing"

// TestRefusedOptionalNvidiaShape: NVIDIA (and other litellm-based relays)
// turn a request away with "Unsupported parameter(s): `enable_thinking`" —
// the plural "(s)" and a backtick-quoted name. refusedOptional must read
// that as a refusal of enable_thinking, so the request is asked again
// without it instead of the 400 reaching the agent (the bug: all 31 keys
// of one provider 400'd on it).
func TestRefusedOptionalNvidiaShape(t *testing.T) {
	body := []byte(`{"model":"m","enable_thinking":true,"messages":[]}`)
	for _, msg := range []string{
		"Validation: Unsupported parameter(s): `enable_thinking`",
		"Unsupported parameter(s): 'enable_thinking'",
		`Unsupported parameter: "enable_thinking"`,
		"unsupported parameter: enable_thinking",
	} {
		got := refusedOptional(400, []byte(msg), body)
		if len(got) != 1 || got[0] != "enable_thinking" {
			t.Errorf("%q -> %v, want [enable_thinking]", msg, got)
		}
	}
	// a field not in the body is not taken for one that is
	if got := refusedOptional(400, []byte("Unsupported parameter(s): `enable_thinking`"), []byte(`{"model":"m"}`)); len(got) != 0 {
		t.Errorf("a field not sent was reported: %v", got)
	}
	// a value's refusal (not the field's) isn't a field to drop
	if got := refusedOptional(400, []byte(`{"error":{"message":"'high' is not supported for 'reasoning_effort' on this model"}}`), body); len(got) != 0 {
		t.Errorf("a value refusal was taken as a field: %v", got)
	}
}