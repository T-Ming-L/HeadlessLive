package bilibili

import "testing"

func TestSign(t *testing.T) {
	// 与 Python 版算法对照：排序后 urlencode + appsec 做 MD5
	params := map[string]string{
		"room_id":  "12345",
		"platform": "web_electron_link",
		"area_v2":  "369",
		"csrf":     "abc",
		"ts":       "1700000000",
		"build":    pcLinkBuild,
		"appkey":   AppKey,
	}
	got := sign(params)
	if len(got) != 32 {
		t.Fatalf("签名长度应为 32，实际 %d", len(got))
	}
	// 签名必须稳定
	if got != sign(params) {
		t.Errorf("同一参数两次签名不一致")
	}
	// 参数顺序不影响结果
	if got != sign(map[string]string{
		"build":    pcLinkBuild,
		"appkey":   AppKey,
		"ts":       "1700000000",
		"csrf":     "abc",
		"area_v2":  "369",
		"platform": "web_electron_link",
		"room_id":  "12345",
	}) {
		t.Errorf("参数顺序改变后签名不一致")
	}
	// 任一参数变化签名都应变化
	params["ts"] = "1700000001"
	if got == sign(params) {
		t.Errorf("参数变化后签名不应相同")
	}
}

func TestCookieRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := NewClient(dir + "/session.json")
	c.mu.Lock()
	c.cookies["SESSDATA"] = "sess"
	c.cookies["bili_jct"] = "jct"
	c.saveLocked()
	c.mu.Unlock()

	// 新客户端应能加载
	c2 := NewClient(dir + "/session.json")
	c2.mu.Lock()
	if c2.cookies["SESSDATA"] != "sess" || c2.cookies["bili_jct"] != "jct" {
		t.Errorf("会话加载失败: %+v", c2.cookies)
	}
	c2.mu.Unlock()

	// CookieString 应包含两项
	s := c2.CookieString()
	if s == "" || len(s) < len("SESSDATA=sess; bili_jct=jct") {
		t.Errorf("CookieString 异常: %s", s)
	}
}
