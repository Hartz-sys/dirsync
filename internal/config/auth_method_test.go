package config

import "testing"

func TestResolvedAuthMethod(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "explicit password",
			cfg:  Config{AuthMethod: "password", Password: "x", PrivateKey: "/tmp/key"},
			want: AuthMethodPassword,
		},
		{
			name: "explicit key",
			cfg:  Config{AuthMethod: "key", Password: "x", PrivateKey: "/tmp/key"},
			want: AuthMethodKey,
		},
		{
			name: "auto prefers key",
			cfg:  Config{AuthMethod: "auto", Password: "x", PrivateKey: "/tmp/key"},
			want: AuthMethodKey,
		},
		{
			name: "auto falls back to password",
			cfg:  Config{AuthMethod: "", Password: "x"},
			want: AuthMethodPassword,
		},
		{
			name: "alias private_key",
			cfg:  Config{AuthMethod: "private_key", PrivateKey: "/tmp/key"},
			want: AuthMethodKey,
		},
	}

	for _, tc := range cases {
		if got := tc.cfg.ResolvedAuthMethod(); got != tc.want {
			t.Fatalf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}
