-- 0003_ledger_user_agent.sql：账单记录登记时的原始 User-Agent（读取时派生短设备标签，JSON 不外露原文）。
-- NOT NULL + DEFAULT ''：历史行与不记录 UA 的 DrawOp 写入的行均为空串。
ALTER TABLE ledger ADD COLUMN user_agent TEXT NOT NULL DEFAULT '';
