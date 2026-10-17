package config

import "testing"

func TestSettings(t *testing.T) {
	temp := 0.7
	b := Settings{Temperature: &temp, MaxTokens: 200}
	s, e := Resolve(b, Settings{MaxTokens: 300})
	if e != nil || s.MaxTokens != 300 || *s.Temperature != temp || s.ContextSize != 4096 {
		t.Fatal(s, e)
	}
	for _, bad := range []Settings{{MaxTokens: -1}, {ContextSize: 20}, {MaxTokens: 5000}} {
		if _, e = Resolve(b, bad); e == nil {
			t.Fatal("accepted invalid override", bad)
		}
	}
}
func TestValidation(t *testing.T) {
	b := Bot{Schema: 1, ID: "demo", Version: "1.0.0", Description: "demo", Instructions: "help", BaseModel: "base", Budgets: Budgets{1000, 2, 30}}
	if e := b.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"../escape", "UPPER", "a/b", ""} {
		b.ID = id
		if b.Validate() == nil {
			t.Fatal(id)
		}
	}
	var s Settings
	for _, data := range []string{`{"max_tokens":1,"oops":true}`, `{} {}`} {
		if Decode([]byte(data), &s) == nil {
			t.Fatal(data)
		}
	}
}
