package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func open(t *testing.T) *DBs {
	t.Helper()
	dbs, err := Open(filepath.Join(t.TempDir(), "t.db"), 5)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = dbs.Close() })
	return dbs
}

func seedTable(dbs *DBs, val int64) error {
	return dbs.WithTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("CREATE TABLE IF NOT EXISTS t (v INTEGER)"); err != nil {
			return err
		}
		_, err := tx.Exec("INSERT INTO t (v) VALUES (?)", val)
		return err
	})
}

func TestPragmasEffective(t *testing.T) {
	dbs := open(t)

	var mode string
	if err := dbs.Write.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}

	var n int64
	if err := dbs.Write.QueryRow("PRAGMA wal_autocheckpoint").Scan(&n); err != nil {
		t.Fatalf("wal_autocheckpoint: %v", err)
	}
	if n != 0 {
		t.Errorf("wal_autocheckpoint = %d, want 0", n)
	}

	if err := dbs.Write.QueryRow("PRAGMA synchronous").Scan(&n); err != nil {
		t.Fatalf("synchronous: %v", err)
	}
	if n != 1 {
		t.Errorf("synchronous = %d, want 1 (NORMAL)", n)
	}

	if err := dbs.Write.QueryRow("PRAGMA mmap_size").Scan(&n); err != nil {
		t.Fatalf("mmap_size: %v", err)
	}
	if n != 134217728 {
		t.Errorf("mmap_size = %d, want 134217728", n)
	}

	if err := dbs.Write.QueryRow("PRAGMA cache_size").Scan(&n); err != nil {
		t.Fatalf("cache_size: %v", err)
	}
	if n != -32000 {
		t.Errorf("cache_size = %d, want -32000", n)
	}

	if err := dbs.Read.QueryRow("PRAGMA query_only").Scan(&n); err != nil {
		t.Fatalf("read query_only: %v", err)
	}
	if n != 1 {
		t.Errorf("read pool query_only = %d, want 1", n)
	}
}

func TestReadPoolCannotWrite(t *testing.T) {
	dbs := open(t)
	if err := seedTable(dbs, 1); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err := dbs.Read.Exec("INSERT INTO t (v) VALUES (2)")
	if err == nil {
		t.Fatal("读池 INSERT 应当失败")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "readonly") && !strings.Contains(msg, "read-only") {
		t.Errorf("错误应为 readonly 类，实际: %v", err)
	}
}

func TestWriteTxCommit(t *testing.T) {
	dbs := open(t)
	if err := seedTable(dbs, 42); err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	var v int64
	if err := dbs.Read.QueryRow("SELECT v FROM t").Scan(&v); err != nil {
		t.Fatalf("读池查询: %v", err)
	}
	if v != 42 {
		t.Errorf("v = %d, want 42", v)
	}
}

func TestReadPoolConcurrent(t *testing.T) {
	dbs := open(t)
	if err := seedTable(dbs, 7); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 6*20)
	for g := 0; g < 6; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				var v int64
				if err := dbs.Read.QueryRow("SELECT v FROM t").Scan(&v); err != nil {
					errCh <- err
					return
				}
				if v != 7 {
					errCh <- fmt.Errorf("v = %d, want 7", v)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		t.Error(e)
	}
}
