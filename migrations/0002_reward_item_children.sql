-- 0002_reward_item_children.sql：积分项适用孩子（设计文档 2026-11-27-point-item-child-scope-design.md §4.1）。
-- 一个积分项没有任何关联行 = 对所有孩子生效；有行 = 恰好对这些孩子生效。
-- 停用孩子不级联清理本表（否则"只给这个孩子"的项会悄悄变成"所有孩子"）。

CREATE TABLE reward_item_children (
  reward_item_id INTEGER NOT NULL REFERENCES reward_items(id),
  child_id       INTEGER NOT NULL REFERENCES users(id),
  PRIMARY KEY (reward_item_id, child_id)
);
CREATE INDEX idx_reward_item_children_child ON reward_item_children(child_id);
