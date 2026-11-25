package auth

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"good-habit/internal/db"
	"good-habit/internal/ledger"
	"good-habit/internal/store"
	"good-habit/migrations"
)

func newService(t *testing.T, lockWindow time.Duration) (*Service, *db.DBs, *store.Store, chan ledger.Op) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "t.db"), 5)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = dbs.Close() })
	if err := dbs.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st := store.New()
	ops := make(chan ledger.Op, 32)
	svc := &Service{DBs: dbs, Store: st, Enqueue: func(op ledger.Op) { ops <- op }, LockWindow: lockWindow}
	return svc, dbs, st, ops
}

func TestRegisterAndLogin(t *testing.T) {
	svc, dbs, st, ops := newService(t, 10*time.Minute)

	sess, err := svc.Register("Dad01", "password8", "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// 用户名规范化为小写、家庭名缺省
	var famName string
	var uname, role string
	err = dbs.Read.QueryRow("SELECT name FROM families WHERE id=?", sess.FamilyID).Scan(&famName)
	if err != nil || famName != "我们家" {
		t.Errorf("家庭名 = %q err=%v, want 我们家", famName, err)
	}
	err = dbs.Read.QueryRow("SELECT username, role FROM users WHERE id=?", sess.UserID).Scan(&uname, &role)
	if err != nil || uname != "dad01" || role != "parent" {
		t.Errorf("users 行不符: %s/%s err=%v, want dad01/parent", uname, role, err)
	}

	// 会话入内存 + 异步落盘事件入队
	if got, ok := st.GetSession(sess.TokenHash); !ok || got.UserID != sess.UserID || got.Role != "parent" {
		t.Errorf("内存会话不符: %+v ok=%v", got, ok)
	}
	select {
	case op := <-ops:
		up, ok := op.(ledger.SessionUpsertOp)
		if !ok || up.TokenHash != sess.TokenHash || up.UserID != sess.UserID {
			t.Errorf("入队事件不符: %+v", op)
		}
	default:
		t.Error("Register 应入队 SessionUpsertOp")
	}

	// 登录大小写不敏感
	sess2, err := svc.Login("DAD01", "password8")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if sess2.UserID != sess.UserID {
		t.Errorf("Login UserID = %d, want %d", sess2.UserID, sess.UserID)
	}
	if sess.Token == "" || sess2.Token == "" {
		t.Error("会话应携带一次性 Token 供写 cookie")
	}
}

func TestDuplicateUsername(t *testing.T) {
	svc, _, _, _ := newService(t, 10*time.Minute)
	if _, err := svc.Register("dad", "password8", ""); err != nil {
		t.Fatalf("首次注册: %v", err)
	}
	if _, err := svc.Register("dad", "password8", "另一家"); err != ErrDuplicateUsername {
		t.Errorf("二次注册 err = %v, want ErrDuplicateUsername", err)
	}
}

func TestLoginLockout(t *testing.T) {
	svc, _, _, _ := newService(t, 50*time.Millisecond)
	if _, err := svc.Register("kid", "password8", ""); err != nil {
		t.Fatalf("Register: %v", err)
	}

	for i := 0; i < 5; i++ {
		if _, err := svc.Login("kid", "wrong-pw-xx"); err != ErrBadCredentials {
			t.Fatalf("第 %d 次错密码 err = %v, want ErrBadCredentials", i+1, err)
		}
	}
	// 锁定窗口内，即使密码正确也 ErrLocked
	if _, err := svc.Login("kid", "password8"); err != ErrLocked {
		t.Errorf("锁定期内正确密码 err = %v, want ErrLocked", err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := svc.Login("kid", "password8"); err != nil {
		t.Errorf("窗口过后应可登录: %v", err)
	}
}

func TestTokenShape(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if len(token) != 43 {
		t.Errorf("token 长度 = %d, want 43 (base64url 32B)", len(token))
	}
	for _, c := range token {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			t.Errorf("token 含非 base64url 字符: %q", c)
		}
	}
	if len(hash) != 64 {
		t.Errorf("hash 长度 = %d, want 64", len(hash))
	}
	for _, c := range hash {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			t.Errorf("hash 含非 hex 字符: %q", c)
		}
	}
}

func TestValidation(t *testing.T) {
	svc, _, _, _ := newService(t, 10*time.Minute)

	for _, bad := range []string{"ab", "bad name!", "中文名too", "x_y_z_01234567890123456789012345678901"} {
		if _, err := svc.Register(bad, "password8", ""); err != ErrValidation {
			t.Errorf("用户名 %q err = %v, want ErrValidation", bad, err)
		}
	}
	if _, err := svc.Register("goodname", "short", ""); err != ErrValidation {
		t.Errorf("短密码 err = %v, want ErrValidation", err)
	}
	// 大写输入 → 规范化为小写，合法（spec §3 存储统一小写）
	if _, err := svc.Register("MiXeD", "password8", ""); err != nil {
		t.Errorf("大写输入应规范化通过: %v", err)
	}
}

func TestHashPassword(t *testing.T) {
	h, err := HashPassword("password8")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	cost, err := bcrypt.Cost([]byte(h))
	if err != nil {
		t.Fatalf("bcrypt.Cost: %v", err)
	}
	if cost != 10 {
		t.Errorf("bcrypt cost = %d, want 10", cost)
	}
	if !CheckPassword(h, "password8") {
		t.Error("正确密码应通过")
	}
	if CheckPassword(h, "password9") {
		t.Error("错误密码不应通过")
	}
}
