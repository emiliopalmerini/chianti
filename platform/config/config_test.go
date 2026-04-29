package config_test

import (
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/emiliopalmerini/chianti/platform/config"
)

func TestGetEnv(t *testing.T) {
	cases := []struct {
		name string
		set  bool
		val  string
		def  string
		want string
	}{
		{"unset returns default", false, "", "fallback", "fallback"},
		{"set non-empty returns value", true, "hello", "fallback", "hello"},
		{"set empty returns default", true, "", "fallback", "fallback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const key = "CHIANTI_TEST_VAR"
			if tc.set {
				t.Setenv(key, tc.val)
			} else {
				t.Setenv(key, "")
				_ = key
			}
			got := config.GetEnv(key, tc.def)
			if got != tc.want {
				t.Errorf("GetEnv = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRandomKey(t *testing.T) {
	k1 := config.RandomKey()
	k2 := config.RandomKey()

	if len(k1) != 44 {
		t.Errorf("len = %d, want 44 (base64 of 32 bytes)", len(k1))
	}
	raw, err := base64.StdEncoding.DecodeString(k1)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("decoded length = %d, want 32", len(raw))
	}
	if k1 == k2 {
		t.Error("two calls returned the same key (entropy?)")
	}
}

func TestParseAdminSeeds(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []config.AdminSeed
	}{
		{"empty", "", nil},
		{
			"single valid",
			"giada:giada@example.com:secret",
			[]config.AdminSeed{{Username: "giada", Email: "giada@example.com", Password: "secret"}},
		},
		{
			"three valid",
			"a:a@x:1,b:b@x:2,c:c@x:3",
			[]config.AdminSeed{
				{Username: "a", Email: "a@x", Password: "1"},
				{Username: "b", Email: "b@x", Password: "2"},
				{Username: "c", Email: "c@x", Password: "3"},
			},
		},
		{
			"skips two-part malformed entry",
			"good:g@x:1,bad:incomplete,also:fine:2",
			[]config.AdminSeed{
				{Username: "good", Email: "g@x", Password: "1"},
				{Username: "also", Email: "fine", Password: "2"},
			},
		},
		{
			"skips four-part malformed entry",
			"good:g@x:1,too:many:parts:here,also:fine:2",
			[]config.AdminSeed{
				{Username: "good", Email: "g@x", Password: "1"},
				{Username: "also", Email: "fine", Password: "2"},
			},
		},
		{"all malformed yields nil", "bad,worse,terrible", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := config.ParseAdminSeeds(tc.raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
