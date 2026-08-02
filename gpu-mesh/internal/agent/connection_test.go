package agent

import "testing"

func TestNormalizeRelayURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// 裸 host → 补端口 + path
		{"192.168.1.100", "ws://192.168.1.100:7780/agent"},
		// host:port → 补 path
		{"192.168.1.100:9000", "ws://192.168.1.100:9000/agent"},
		// ws://host:port 无 path → 补 /agent
		{"ws://relay.example.com:7780", "ws://relay.example.com:7780/agent"},
		// ws://host 无端口无 path → 补默认端口？否，ws:// 开头不补端口，只补 path
		{"ws://relay.example.com", "ws://relay.example.com/agent"},
		// 已有 path → 原样
		{"ws://h:7780/agent", "ws://h:7780/agent"},
		{"wss://h:443/agent", "wss://h:443/agent"},
		{"wss://h/custom", "wss://h/custom"},
		// wss:// 无 path
		{"wss://h", "wss://h/agent"},
		// 空
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeRelayURL(c.in)
		if got != c.want {
			t.Errorf("normalizeRelayURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAppendToken(t *testing.T) {
	// 空 token 不附加
	if got := appendToken("ws://h/agent", ""); got != "ws://h/agent" {
		t.Errorf("空 token 应原样，得到 %s", got)
	}
	// 有 token 附加查询参数
	got := appendToken("ws://h:7780/agent", "s3cret")
	if got != "ws://h:7780/agent?token=s3cret" {
		t.Errorf("token 附加错误，得到 %s", got)
	}
	// 已有查询参数则追加
	got = appendToken("ws://h/agent?foo=bar", "tk")
	if got != "ws://h/agent?foo=bar&token=tk" {
		t.Errorf("追加到已有查询参数错误，得到 %s", got)
	}
}
