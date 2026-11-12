package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func FuzzDecodeRule(f *testing.F) {
	seeds := []string{
		`{"name":"r1","priority":1,"enabled":true,"conditions":{"description_contains":"amazon"},"actions":{"category_id":1}}`,
		`{"name":"r2","priority":2,"enabled":true,"conditions":{"amount":{"min_abs_minor":100,"max_abs_minor":1000,"sign":"outflow"}},"actions":{"payee_id":7,"tag_ids":[10,11]}}`,
		`{"name":"r3","priority":3,"enabled":false,"conditions":{"account_ids":[1,2]},"actions":{"category_id":3}}`,
		`{"name":"bad","priority":1,"enabled":true,"conditions":{"regex":"x"},"actions":{"category_id":1}}`,
		`{"name":"","priority":1,"enabled":true,"conditions":{},"actions":{"category_id":1}}`,
		`{"name":"no-actions","priority":1,"enabled":true,"conditions":{},"actions":{}}`,
		`{"name":"bad-sign","priority":1,"enabled":true,"conditions":{"amount":{"sign":"regex"}},"actions":{"category_id":1}}`,
		`{"name":"unknown","priority":1,"enabled":true,"conditions":{},"actions":{"category_id":1},"unknown":1}`,
		`{`,
		``,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		var r Rule
		err := json.Unmarshal([]byte(input), &r)
		if err != nil {
			var de *Error
			if errors.As(err, &de) {
				return
			}
			var syn *json.SyntaxError
			if errors.As(err, &syn) {
				return
			}
			var typ *json.UnmarshalTypeError
			if errors.As(err, &typ) {
				return
			}
			return
		}

		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var out Rule
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("round-trip unmarshal: %v", err)
		}
	})
}
