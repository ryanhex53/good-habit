// Package db 显式拆分 SQLite 读写两个连接池（设计文档 §6.2）：
// WriteDB 单连接 + _txlock=immediate + Go 层互斥锁；ReadDB 只读 + query_only，
// 4–6 连接并发读。打开顺序固定：先写池（建立 WAL/-shm），后读池。
package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，无 CGO
)

// DBs 聚合读写两池；所有写事务都必须经 WithTx（内部锁 WriteMu）。
type DBs struct {
	Write   *sql.DB
	Read    *sql.DB
	WriteMu sync.Mutex
}

// WriteDSN 是写池连接串：全部 PRAGMA 基线来自设计文档 §6.2。
func WriteDSN(dbPath string) string {
	p := filepath.ToSlash(dbPath)
	return "file:" + p +
		"?_txlock=immediate" +
		"&_pragma=busy_timeout(10000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=mmap_size(134217728)" +
		"&_pragma=cache_size(-32000)" +
		"&_pragma=wal_autocheckpoint(0)"
}

// ReadDSN 是读池连接串：物理只读 + query_only 双保险。
func ReadDSN(dbPath string) string {
	p := filepath.ToSlash(dbPath)
	return "file:" + p +
		"?mode=ro" +
		"&_pragma=query_only(1)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=mmap_size(134217728)" +
		"&_pragma=cache_size(-32000)"
}

// Open 先打开写池（Ping 立刻建立连接、应用 PRAGMA、生成 WAL/-shm），
// 再打开读池，避免只读连接遇到需要恢复的 WAL。
func Open(dbPath string, readPool int) (*DBs, error) {
	write, err := sql.Open("sqlite", WriteDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("打开写池: %w", err)
	}
	write.SetMaxOpenConns(1)
	write.SetMaxIdleConns(1)
	if err := write.Ping(); err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("写池 Ping: %w", err)
	}

	read, err := sql.Open("sqlite", ReadDSN(dbPath))
	if err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("打开读池: %w", err)
	}
	read.SetMaxOpenConns(readPool)
	read.SetMaxIdleConns(readPool)
	if err := read.Ping(); err != nil {
		_ = read.Close()
		_ = write.Close()
		return nil, fmt.Errorf("读池 Ping: %w", err)
	}

	return &DBs{Write: write, Read: read}, nil
}

// WithTx 锁 WriteMu → BEGIN（连接为 _txlock=immediate，即取写锁）→ fn → 提交/回滚。
// 一切同步写路径（家长 CRUD、迁移、组提交批次）都必须走这里，杜绝 SQLITE_BUSY。
func (d *DBs) WithTx(fn func(tx *sql.Tx) error) error {
	d.WriteMu.Lock()
	defer d.WriteMu.Unlock()

	tx, err := d.Write.Begin()
	if err != nil {
		return fmt.Errorf("BEGIN: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("COMMIT: %w", err)
	}
	return nil
}

// Close 依次关闭读写池。
func (d *DBs) Close() error {
	var firstErr error
	if err := d.Read.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := d.Write.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
