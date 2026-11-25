package ledger

import (
	"crypto/rand"
	"encoding/hex"
)

// NewUUID 生成 32 位 hex 随机标识（ledger/draws 的 uuid 列）。
// crypto/rand 失败属系统级不可恢复状态，直接 panic。
func NewUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("ledger: crypto/rand 失败: " + err.Error())
	}
	return hex.EncodeToString(b)
}
