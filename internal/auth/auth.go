// Package auth 提供注册/登录/登出与密码、令牌、失败锁定（设计文档 §10）。
// 会话常驻内存（store），经异步组提交落盘，进程重启不掉登录态。
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"good-habit/internal/db"
	"good-habit/internal/ledger"
	"good-habit/internal/store"
)

// SessionTTL 是会话有效期（30 天滑动）。
const SessionTTL = 30 * 24 * time.Hour

const maxLoginFails = 5

var (
	ErrDuplicateUsername = errors.New("duplicate_username")
	ErrBadCredentials    = errors.New("bad_credentials")
	ErrLocked            = errors.New("account_locked")
	ErrInactive          = errors.New("account_inactive")
	ErrValidation        = errors.New("invalid_input")
)

var usernameRe = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

// Service 承载注册/登录/登出。LockWindow 为同账号失败锁定时长（测试可注入）。
type Service struct {
	DBs     *db.DBs
	Store   *store.Store
	Enqueue func(ledger.Op)

	LockWindow time.Duration

	lockMu  sync.Mutex
	fails   map[string]int       // 小写用户名 → 连续失败次数
	lockedU map[string]int64     // 小写用户名 → 锁定截止（Unix 毫秒）
}

// hashPassword 用 bcrypt cost 10 散列。
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(b), nil
}

// CheckPassword 校验密码。
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// NewToken 生成 32 字节随机令牌：token 给客户端（base64url 43 字符），
// tokenHash = sha256(token) hex 入库/内存。
func NewToken() (token, tokenHash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("rand: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

func (a *Service) lockWindow() time.Duration {
	if a.LockWindow <= 0 {
		return 10 * time.Minute
	}
	return a.LockWindow
}

func (a *Service) initLock() {
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	if a.fails == nil {
		a.fails = map[string]int{}
	}
	if a.lockedU == nil {
		a.lockedU = map[string]int64{}
	}
}

// Register 注册家长账号：建 family + parent 用户，并建立会话。
func (a *Service) Register(username, password, familyName string) (store.Session, error) {
	uname := strings.ToLower(strings.TrimSpace(username))
	if !usernameRe.MatchString(uname) {
		return store.Session{}, ErrValidation
	}
	if len(password) < 8 {
		return store.Session{}, ErrValidation
	}
	familyName = strings.TrimSpace(familyName)
	if familyName == "" {
		familyName = "我们家"
	}

	hash, err := HashPassword(password)
	if err != nil {
		return store.Session{}, err
	}
	now := time.Now().UnixMilli()

	var userID, familyID int64
	err = a.DBs.WithTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			"INSERT INTO families (name, created_at) VALUES (?,?)", familyName, now)
		if err != nil {
			return fmt.Errorf("insert family: %w", err)
		}
		if familyID, err = res.LastInsertId(); err != nil {
			return err
		}
		res, err = tx.Exec(
			`INSERT INTO users (family_id, role, username, password_hash, display_name, created_at)
			 VALUES (?,?,?,?,?,?)`,
			familyID, "parent", uname, hash, uname, now)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return ErrDuplicateUsername
			}
			return fmt.Errorf("insert user: %w", err)
		}
		userID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return store.Session{}, err
	}
	return a.establishSession(userID, familyID, "parent")
}

// Login 校验用户名密码；同账号连续失败 5 次锁定一个 LockWindow。
func (a *Service) Login(username, password string) (store.Session, error) {
	uname := strings.ToLower(strings.TrimSpace(username))
	if uname == "" || password == "" {
		return store.Session{}, ErrValidation
	}
	a.initLock()

	a.lockMu.Lock()
	if until, locked := a.lockedU[uname]; locked {
		if time.Now().UnixMilli() < until {
			a.lockMu.Unlock()
			return store.Session{}, ErrLocked
		}
		delete(a.lockedU, uname)
		delete(a.fails, uname)
	}
	a.lockMu.Unlock()

	var (
		userID, familyID   int64
		role, passwordHash string
		active             int64
	)
	err := a.DBs.Read.QueryRow(
		"SELECT id, family_id, role, password_hash, active FROM users WHERE username=?", uname,
	).Scan(&userID, &familyID, &role, &passwordHash, &active)
	if errors.Is(err, sql.ErrNoRows) {
		// 不区分"不存在"，避免用户名探测
		a.noteFail(uname)
		return store.Session{}, ErrBadCredentials
	}
	if err != nil {
		return store.Session{}, fmt.Errorf("查用户: %w", err)
	}
	if active == 0 {
		// 停用账号：明确告知（孩子知道自己是被停用的），不计失败次数
		return store.Session{}, ErrInactive
	}
	if !CheckPassword(passwordHash, password) {
		a.noteFail(uname)
		return store.Session{}, ErrBadCredentials
	}

	a.lockMu.Lock()
	delete(a.fails, uname)
	delete(a.lockedU, uname)
	a.lockMu.Unlock()

	return a.establishSession(userID, familyID, role)
}

// Logout 删除会话（内存 + 异步落盘）。
func (a *Service) Logout(tokenHash string) {
	a.Store.DeleteSession(tokenHash)
	a.Enqueue(ledger.SessionDeleteOp{TokenHash: tokenHash})
}

func (a *Service) establishSession(userID, familyID int64, role string) (store.Session, error) {
	token, tokenHash, err := NewToken()
	if err != nil {
		return store.Session{}, err
	}
	now := time.Now().UnixMilli()
	sess := store.Session{
		TokenHash: tokenHash,
		UserID:    userID,
		FamilyID:  familyID,
		Role:      role,
		ExpiresAt: now + SessionTTL.Milliseconds(),
	}
	a.Store.PutSession(sess)
	a.Enqueue(ledger.SessionUpsertOp{
		TokenHash: tokenHash, UserID: userID,
		ExpiresAt: sess.ExpiresAt, CreatedAt: now,
	})
	// 会话载荷带 token 本体，供 handler 写 cookie；TokenHash 是持久身份。
	sess.TokenHash = tokenHash
	sess.Token = token
	return sess, nil
}

func (a *Service) noteFail(uname string) {
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	a.fails[uname]++
	if a.fails[uname] >= maxLoginFails {
		a.lockedU[uname] = time.Now().Add(a.lockWindow()).UnixMilli()
		delete(a.fails, uname)
	}
}
