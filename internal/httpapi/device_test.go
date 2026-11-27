package httpapi

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDeviceLabel(t *testing.T) {
	cases := []struct{ name, ua, want string }{
		{"iPhone Safari", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", "iPhone iOS 17.5"},
		{"iPhone 微信内置（无 Version/、含 MicroMessenger）", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 MicroMessenger/8.0.47(0x18002f2f) NetType/WIFI Language/zh_CN", "iPhone iOS 16.6"},
		{"iPhone 三段版本", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1", "iPhone iOS 18.1.1"},
		{"iPad", "Mozilla/5.0 (iPad; CPU OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", "iPad iPadOS 17.5"},
		{"iPod", "Mozilla/5.0 (iPod; CPU iPhone OS 15_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.6 Mobile/15E148 Safari/604.1", "iPod iOS 15.7"},
		{"Pixel", "Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", "Pixel 8 Pro Android 14"},
		{"华为 + Build/ + 微信 wv", "Mozilla/5.0 (Linux; Android 12; ALN-AL00 Build/HUAWEIALN-AL00; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/116.0.0.0 Mobile Safari/537.36 XWEB/1160065 MMWEBSDK/20231202 MicroMessenger/8.0.47.2560(0x28003036) WeChat/arm64 Weixin NetType/5G Language/zh_CN ABI/arm64", "ALN-AL00 Android 12"},
		{"小米纯数字型号", "Mozilla/5.0 (Linux; Android 13; 2211133C Build/TP1A.220624.014) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Mobile Safari/537.36", "2211133C Android 13"},
		{"三星带旧版本号", "Mozilla/5.0 (Linux; Android 6.0.1; SM-G920F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/76.0.3809.132 Mobile Safari/537.36", "SM-G920F Android 6.0.1"},
		{"Chrome 缩减形态 Android 10; K → 版本被冻结，不显示误导性版本", "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36", "Android"},
		{"Android 无型号", "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", "Android 14"},
		{"Windows Chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Windows"},
		{"macOS Safari", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15", "macOS"},
		{"ChromeOS（含 X11，须先于 X11 命中）", "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "ChromeOS"},
		{"Linux 桌面", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", "Linux"},
		{"空串", "", ""},
		{"只有空白", "   ", ""},
		{"无法识别", "curl/8.4.0", ""},
	}
	for _, c := range cases {
		if got := deviceLabel(c.ua); got != c.want {
			t.Errorf("%s: deviceLabel = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestClipUserAgent(t *testing.T) {
	if got := clipUserAgent(""); got != "" {
		t.Errorf("空串 → %q", got)
	}
	if got := clipUserAgent("  ua-x \n"); got != "ua-x" {
		t.Errorf("去首尾空白 → %q", got)
	}
	if got := clipUserAgent(strings.Repeat("a", 600)); len(got) != maxUserAgentBytes {
		t.Errorf("600 ASCII → %d 字节, want %d", len(got), maxUserAgentBytes)
	}
	if got := clipUserAgent(strings.Repeat("a", maxUserAgentBytes)); got != strings.Repeat("a", maxUserAgentBytes) {
		t.Errorf("恰好 %d 字节不应截断 → %d 字节", maxUserAgentBytes, len(got))
	}
	// 多字节字符不能被切成半个 rune："中" 3 字节，512 以内最多 170 个 = 510 字节
	got := clipUserAgent(strings.Repeat("中", 300))
	if len(got) != 510 || !utf8.ValidString(got) {
		t.Errorf("300×中 → %d 字节 valid=%v, want 510 / true", len(got), utf8.ValidString(got))
	}
}
