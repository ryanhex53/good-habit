// Package seed 提供一键导入的引导种子数据（设计文档附录 A 原文）。
package seed

import "good-habit/internal/store"

// Data 返回默认积分项（11 项）与转盘奖品（3 项）。
func Data() (items []store.RewardItem, prizes []store.Prize) {
	items = []store.RewardItem{
		{Title: "托管课写完作业", Points: 7},
		{Title: "上喜课经过审核", Points: 5},
		{Title: "额外作业", Points: 10},
		{Title: "被老师表扬", Points: 5},
		{Title: "考试单科第一", Points: 10},
		{Title: "1个鸡蛋", Points: 5},
		{Title: "1盒牛奶", Points: 5},
		{Title: "720完成起床", Points: 5},
		{Title: "早读10分钟", Points: 5},
		{Title: "945上床睡觉", Points: 5},
		{Title: "背诵1篇英语短文", Points: 5},
	}
	prizes = []store.Prize{
		{Name: "水果糖1颗", Emoji: "🍬"},
		{Name: "巧克力1颗", Emoji: "🍫"},
		{Name: "自选零食1袋", Emoji: "🥨"},
	}
	return items, prizes
}
