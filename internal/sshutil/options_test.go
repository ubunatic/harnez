package sshutil

import "testing"

func TestValidHost(t *testing.T) {
	for _, test := range []struct {
		host string
		want bool
	}{
		{"x600", true}, {"user@host", true}, {"-host", false}, {"-oProxyCommand=touch", false},
		{"host\ncommand", false}, {"host name", false}, {"host;id", false}, {"", false},
	} {
		if got := ValidHost(test.host); got != test.want {
			t.Errorf("ValidHost(%q) = %v, want %v", test.host, got, test.want)
		}
	}
}

func TestValidInstallPath(t *testing.T) {
	for _, test := range []struct {
		path string
		want bool
	}{
		{"~/go/bin/harnez", true}, {"~/x600", true}, {"~/../tmp/x", false}, {"~/a b", false},
		{"~/a\nb", false}, {"~/a;b", false}, {"~/$(id)", false}, {"~/a*", false},
		{"/opt/harnez/bin/harnez", true}, {"/usr/local/bin/harnez", true}, {"/workspace/bin/harnez", true},
		{"/tmp/x", false}, {"/tmp", false}, {"/var/tmp/x", false}, {"/var/tmp", false},
		{"/a/../b", false}, {"/./tmp/x", false}, {"/var/./tmp/x", false}, {"~/./x", false}, {"/a/./b", false},
		{"/a b", false}, {"/a;b", false}, {"//x", false}, {"bin/x", false},
	} {
		if got := ValidInstallPath(test.path); got != test.want {
			t.Errorf("ValidInstallPath(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}
