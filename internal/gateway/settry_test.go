package gateway

import "testing"

// setTry 把 try 写进"代表它的那一行"（ID 相同的最后一行），而不是
// 盲写最后一行：一场 race 里每条 lane 各带一行，盲写最后一行会抹掉
// 最后一条 lane 的结局、把胜者画两遍——旧惯用法的病根。
func TestSetTryMatchesByID(t *testing.T) {
	tr := &Route{Tries: []Try{
		{ID: "a", Done: true, Status: 408},
		{ID: "b", Done: true, Status: 200},
	}}
	// 胜者是 a（不是最后一行）：写 a 的行，b 的行不动
	setTry(tr, Try{ID: "a", Done: true, Status: 200, Millis: 900})
	if tr.Tries[0].Status != 200 || tr.Tries[0].Millis != 900 {
		t.Fatalf("the winner's row was not replaced: %+v", tr.Tries[0])
	}
	if tr.Tries[1].Status != 200 || tr.Tries[1].ID != "b" {
		t.Fatalf("another lane's row was touched: %+v", tr.Tries[1])
	}
	if len(tr.Tries) != 2 {
		t.Fatalf("rows = %d, want the count kept", len(tr.Tries))
	}
	// 没有可匹配的行时才添新行
	setTry(tr, Try{ID: "c", Done: true, Status: 429})
	if len(tr.Tries) != 3 || tr.Tries[2].ID != "c" {
		t.Fatalf("an unmatched try should append: %+v", tr.Tries[2])
	}
	// 同 ID 多行时写最后一行（后发者）
	setTry(tr, Try{ID: "c", Status: 500, Error: "x"})
	if tr.Tries[2].Status != 500 {
		t.Fatalf("the last matching row should win: %+v", tr.Tries[2])
	}
}
