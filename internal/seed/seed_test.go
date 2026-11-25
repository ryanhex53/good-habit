package seed

import "testing"

func TestData(t *testing.T) {
	items, prizes := Data()

	want := map[string]int64{
		"托管课写完作业": 7,
		"上喜课经过审核": 5,
		"额外作业":     10,
		"被老师表扬":    5,
		"考试单科第一":   10,
		"1个鸡蛋":     5,
		"1盒牛奶":     5,
		"720完成起床":  5,
		"早读10分钟":   5,
		"945上床睡觉":  5,
		"背诵1篇英语短文": 5,
	}
	if len(items) != len(want) {
		t.Fatalf("积分项数量 = %d, want %d", len(items), len(want))
	}
	for _, it := range items {
		if p, ok := want[it.Title]; !ok || p != it.Points {
			t.Errorf("积分项 %q 分值 = %d, want %v", it.Title, it.Points, want[it.Title])
		}
	}

	wantPrizes := map[string]string{
		"水果糖1颗":  "🍬",
		"巧克力1颗":  "🍫",
		"自选零食1袋": "🥨",
	}
	if len(prizes) != len(wantPrizes) {
		t.Fatalf("奖品数量 = %d, want %d", len(prizes), len(wantPrizes))
	}
	for _, pz := range prizes {
		if e, ok := wantPrizes[pz.Name]; !ok || e != pz.Emoji {
			t.Errorf("奖品 %q emoji = %q, want %v", pz.Name, pz.Emoji, wantPrizes[pz.Name])
		}
	}
}
