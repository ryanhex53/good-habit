// Package store 是热路径内存态（设计文档 §6.4）：
// 孩子余额（按孩子细粒度锁）、家庭配置快照（原子指针整体替换）、
// 会话、每家庭"今日得分"计数——全部驻留 Go 进程内存，读路径不穿透磁盘。
package store

import (
	"sync"
	"sync/atomic"
)

// RewardItem 是内存快照里的积分项（active-only）。ChildIDs 为空 = 对所有孩子生效。
type RewardItem struct {
	ID       int64
	Title    string
	Points   int64
	ChildIDs []int64
}

// AppliesTo 报告该项能否记给 childID：未指定孩子（ChildIDs 为空）对所有孩子生效。
func (it RewardItem) AppliesTo(childID int64) bool {
	if len(it.ChildIDs) == 0 {
		return true
	}
	for _, id := range it.ChildIDs {
		if id == childID {
			return true
		}
	}
	return false
}

// Prize 是内存快照里的转盘奖品（active-only）。
type Prize struct {
	ID    int64
	Name  string
	Emoji string
}

// FamilyConfig 是一个家庭的完整热配置副本：读无锁、写整体替换。
type FamilyConfig struct {
	FamilyID   int64
	Items      []RewardItem
	Prizes     []Prize
	SingleCost int64
	DoubleCost int64
}

// Session 是内存会话。TokenHash 是持久身份（DB 主键）；
// Token 仅在 auth 建会话时随返回值带给 handler 写 cookie，不入库、不驻留。
type Session struct {
	TokenHash string
	UserID    int64
	FamilyID  int64
	Role      string // "parent" | "child"
	ExpiresAt int64
	Token     string // 传递用：仅 establishSession 返回时非空
}

// ChildAcct 是一个孩子账户的热状态。Balance/PendingUUID/Unfulfilled
// 的"检查+变更+入队"必须在 mu 内原子完成（由 httpapi 层遵守）。
type ChildAcct struct {
	mu              sync.Mutex
	Balance         int64  // 内存权威余额，可能领先 DB 一个提交窗口
	PendingUUID     string // 双抽待选局（至多一局；空 = 无）
	PendingDeadline int64  // 48h 超时（Unix 毫秒）
	Unfulfilled     int64  // 已中奖待发放数（家长总览用）
}

// Lock 拿到孩子互斥锁；热路径写余额/待选态的唯一入口。
func (c *ChildAcct) Lock() { c.mu.Lock() }

// Unlock 释放孩子互斥锁。
func (c *ChildAcct) Unlock() { c.mu.Unlock() }

// Bal 加锁读余额（展示/列表路径用）。
func (c *ChildAcct) Bal() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Balance
}

// Stats 加锁读全部热状态（测试与家长总览用）。
func (c *ChildAcct) Stats() (balance int64, pendingUUID string, unfulfilled int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Balance, c.PendingUUID, c.Unfulfilled
}

// Store 聚合全部内存热状态。
type Store struct {
	children  sync.Map // map[int64]*ChildAcct
	sessions  sync.Map // map[string]Session
	families  atomic.Pointer[map[int64]*FamilyConfig]
	todayMu   sync.Mutex
	todayCnts map[int64]map[string]int64 // familyID → dayKey → 得分笔数
}

// New 建空 Store；启动时用 LoadAll 从 DB 重建。
func New() *Store {
	s := &Store{todayCnts: map[int64]map[string]int64{}}
	s.families.Store(&map[int64]*FamilyConfig{})
	return s
}

// Child 懒创建/取回孩子账户。
func (s *Store) Child(id int64) *ChildAcct {
	v, _ := s.children.LoadOrStore(id, &ChildAcct{})
	return v.(*ChildAcct)
}

// Family 取家庭配置快照；未加载返回 nil。
func (s *Store) Family(familyID int64) *FamilyConfig {
	m := *s.families.Load()
	return m[familyID]
}

// SetFamily 整体替换一份家庭配置快照（copy-on-write）。
func (s *Store) SetFamily(fc *FamilyConfig) {
	for {
		oldPtr := s.families.Load()
		next := make(map[int64]*FamilyConfig, len(*oldPtr)+1)
		for k, v := range *oldPtr {
			next[k] = v
		}
		next[fc.FamilyID] = fc
		if s.families.CompareAndSwap(oldPtr, &next) {
			return
		}
	}
}

// PutSession 写入内存会话。
func (s *Store) PutSession(sess Session) { s.sessions.Store(sess.TokenHash, sess) }

// GetSession 查内存会话。
func (s *Store) GetSession(tokenHash string) (Session, bool) {
	v, ok := s.sessions.Load(tokenHash)
	if !ok {
		return Session{}, false
	}
	return v.(Session), true
}

// DeleteSession 删除内存会话。
func (s *Store) DeleteSession(tokenHash string) { s.sessions.Delete(tokenHash) }

// DeleteSessionsForUser 删除某用户的全部内存会话（停用账号时调用）。
func (s *Store) DeleteSessionsForUser(userID int64) {
	s.sessions.Range(func(k, v any) bool {
		if sess, ok := v.(Session); ok && sess.UserID == userID {
			s.sessions.Delete(k)
		}
		return true
	})
}

// BumpEarnsToday 给家庭当日得分笔数 +1。
func (s *Store) BumpEarnsToday(familyID int64, dayKey string) {
	s.todayMu.Lock()
	defer s.todayMu.Unlock()
	days, ok := s.todayCnts[familyID]
	if !ok {
		days = map[string]int64{}
		s.todayCnts[familyID] = days
	}
	days[dayKey]++
}

// EarnsToday 读家庭当日得分笔数（无记录为 0）。
func (s *Store) EarnsToday(familyID int64, dayKey string) int64 {
	s.todayMu.Lock()
	defer s.todayMu.Unlock()
	return s.todayCnts[familyID][dayKey]
}
