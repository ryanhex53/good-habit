package httpapi

import (
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

// maxUserAgentBytes 落库的 User-Agent 上限：主流 UA 110–150 字节，微信内置浏览器约 250–400。
// 只在 Go 侧约束（不加 CHECK 约束——违反会让整个 earn op 被组提交隔离）。
const maxUserAgentBytes = 512

// requestUserAgent 读取请求的 User-Agent 并裁剪到 maxUserAgentBytes，供登记得分/手动调整落库。
func requestUserAgent(r *http.Request) string {
	return clipUserAgent(r.UserAgent())
}

// clipUserAgent：TrimSpace 后在 rune 边界截断到 maxUserAgentBytes。
func clipUserAgent(raw string) string {
	s := strings.TrimSpace(raw)
	for len(s) > maxUserAgentBytes {
		_, size := utf8.DecodeLastRuneInString(s) // size>=1 → 必终止；坏字节 size=1
		s = s[:len(s)-size]
	}
	return s
}

var (
	iosRe = regexp.MustCompile(`\((iPhone|iPad|iPod); CPU (?:iPhone OS|OS) ([0-9_]+) like Mac OS X`)
	// 旧式 (Linux; U; Android x; <locale>; <model>) 的 "U;" 与 locale 段都要容忍：小米自带浏览器、
	// QQ/UC/百度浏览器与 Android ≤4.3 仍是这个格式，型号在第三段；漏掉会落到 Linux 规则给出错误的 "Linux"。
	// 组 2 是型号，或是 locale（此时型号在组 3）；[;)] 兼容微信 WebView 的 "; wv)"
	androidRe = regexp.MustCompile(`\(Linux;(?: U;)? Android ([0-9]+(?:\.[0-9]+)*)(?:; ([^;)]+))?(?:; ([^;)]+))?`)
	localeRe  = regexp.MustCompile(`^[a-z]{2,3}(?:-[A-Za-z0-9]{2,8})*$`)
	buildRe   = regexp.MustCompile(`\s*Build/\S+`)
)

// deviceLabel 把原始 User-Agent 解析成短设备标签，供账单展示：
//
//	iPhone   → "iPhone iOS 17.5"（iOS ≥13 的 UA 不含型号，只能到系统版本）
//	Android  → "Pixel 8 Pro Android 14"；Chrome ≥110 的缩减形态 "Android 10; K" 型号与版本都不可信 → "Android"
//	桌面     → "Windows" / "macOS" / "Linux" / "ChromeOS"；无法识别 → ""
//
// 匹配顺序有讲究：iOS 先于 Mac OS X（iOS UA 含 "like Mac OS X"），Android 先于 Linux，CrOS 先于 X11。
func deviceLabel(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return ""
	}
	if m := iosRe.FindStringSubmatch(ua); m != nil {
		osName := "iOS"
		if m[1] == "iPad" {
			osName = "iPadOS"
		}
		return m[1] + " " + osName + " " + strings.ReplaceAll(m[2], "_", ".")
	}
	if m := androidRe.FindStringSubmatch(ua); m != nil {
		raw := m[2]
		if localeRe.MatchString(raw) {
			raw = m[3] // 跳过 zh-cn / en-us 这类 locale 段
		}
		model := strings.TrimSpace(buildRe.ReplaceAllString(raw, ""))
		if model == "K" { // Chrome UA 缩减占位：版本也被冻结成 10，不显示误导性的版本
			return "Android"
		}
		if model == "" || model == "wv" || len(model) > 40 { // 无型号 / WebView 标记 / 垃圾 token
			return "Android " + m[1]
		}
		return model + " Android " + m[1]
	}
	switch {
	case strings.Contains(ua, "Windows NT"):
		return "Windows"
	case strings.Contains(ua, "Mac OS X"):
		return "macOS"
	case strings.Contains(ua, "CrOS"):
		return "ChromeOS"
	case strings.Contains(ua, "X11") || strings.Contains(ua, "Linux"):
		return "Linux"
	}
	return ""
}
