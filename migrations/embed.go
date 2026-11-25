// Package migrations 把 SQL 迁移脚本打进二进制（设计文档 §6.3）。
package migrations

import "embed"

// FS 根目录即全部 *.sql 迁移文件，文件名 <版本>_<名称>.sql 按字典序执行。
//
//go:embed *.sql
var FS embed.FS
