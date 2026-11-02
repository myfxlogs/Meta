package db

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	pbv1 "meta/internal/pb/metav1"
)

// TestClean_Cargs 写径实参映射（票0019 锚2）：string→净化，其余类型原样。
func TestClean_Cargs(t *testing.T) {
	raw := []byte{0xff, 0x80}
	out := cargs("a\xffb", int64(7), raw, "ok", true, nil)
	if got := out[0].(string); !utf8.ValidString(got) || !strings.Contains(got, "�") {
		t.Fatalf("string 实参未净化: %q", got)
	}
	if out[1] != int64(7) {
		t.Fatal("int64 实参被动")
	}
	if b, ok := out[2].([]byte); !ok || b[0] != 0xff {
		t.Fatal("[]byte 实参被动——BLOB 列必须原样")
	}
	if out[3] != "ok" {
		t.Fatal("合法串漂移")
	}
	if out[4] != true || out[5] != nil {
		t.Fatal("非 string 类型被动")
	}
}

// TestSanitize_AppendEvent 入账口净化（票0019 锚2＋喂毒锚5）：毒 note 经
// AppendEvent 入库→Events 回读合法 UTF-8 含 �＋行在（决策 67 毒行 394/543
// 同形态直喂）。
func TestSanitize_AppendEvent(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	if err := d.CreateTicket(ctx, "p1", "毒票", nil); err != nil {
		t.Fatal(err)
	}
	poison := string([]byte{0xff, 0xfe, 'a', 0x80})
	if err := d.AppendEvent(ctx, "p1", "progress", Event{
		Actor: "test", Note: "x" + poison + "y"}); err != nil {
		t.Fatal(err)
	}
	evs, err := d.Events(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	var found *Event
	for _, e := range evs {
		if e.Kind == "progress" {
			found = e
		}
	}
	if found == nil {
		t.Fatal("事件行未落")
	}
	if !utf8.ValidString(found.Note) {
		t.Fatalf("回读 note 仍含毒: %q", found.Note)
	}
	if !strings.Contains(found.Note, "�") {
		t.Fatalf("note 缺 U+FFFD 替换符: %q", found.Note)
	}
}

// TestSanitize_SpecBlob spec BLOB 闸（票0019 锚2＋对抗锚）：伪 wire→UpdateSpec
// 返错且列未写；合法 spec→落库可 Unmarshal。
func TestSanitize_SpecBlob(t *testing.T) {
	d := openTemp(t)
	ctx := context.Background()
	if err := d.CreateTicket(ctx, "s1", "spec 票", nil); err != nil {
		t.Fatal(err)
	}
	// 伪 wire：0x0a=field1(goal) LEN tag + len2 + {0xff,0xfe} 非法 UTF-8。
	bad := []byte{0x0a, 0x02, 0xff, 0xfe}
	if err := d.UpdateSpec(ctx, "s1", bad); err == nil {
		t.Fatal("伪 spec wire 应拒写")
	}
	tk, err := d.GetTicket(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(tk.Spec) != 0 {
		t.Fatalf("坏 spec 竟落库: %x", tk.Spec)
	}
	good, err := proto.Marshal(&pbv1.TicketSpec{Goal: "目标", Why: "为什么",
		Scope: "范围", OutOfScope: "禁线", Refs: "引用", Anchors: "锚",
		Verify: "验证", Deliver: "交付"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateSpec(ctx, "s1", good); err != nil {
		t.Fatalf("合法 spec 应落库: %v", err)
	}
	tk, err = d.GetTicket(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	ts := &pbv1.TicketSpec{}
	if err := proto.Unmarshal(tk.Spec, ts); err != nil {
		t.Fatalf("回读 unmarshal: %v", err)
	}
	if ts.Goal != "目标" {
		t.Fatalf("spec 栏值漂移: %q", ts.Goal)
	}
	// 立票径同闸：CreateTicketProv 伪 spec 整票拒立。
	if err := d.CreateTicketProv(ctx, "s2", "t", bad, "test"); err == nil {
		t.Fatal("CreateTicketProv 伪 spec 应拒")
	}
	if tk, _ := d.GetTicket(ctx, "s2"); tk != nil {
		t.Fatal("伪 spec 票竟落库")
	}
}
