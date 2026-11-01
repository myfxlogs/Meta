package db

import (
	"context"
	"path/filepath"
	"testing"
)

// TestFeishuCardCRUD 票0014：feishu_card 行——无行 nil,nil；落行可读；
// 同键重落幂等覆盖（单票单卡）。
func TestFeishuCardCRUD(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	if c, err := d.GetFeishuCard(ctx, "t1"); err != nil || c != nil {
		t.Fatalf("无行应 nil,nil: c=%v err=%v", c, err)
	}
	if err := d.SaveFeishuCard(ctx, "t1", "om_1", "oc_1"); err != nil {
		t.Fatalf("save: %v", err)
	}
	c, err := d.GetFeishuCard(ctx, "t1")
	if err != nil || c == nil {
		t.Fatalf("落行读不回: c=%v err=%v", c, err)
	}
	if c.MessageID != "om_1" || c.ChatID != "oc_1" {
		t.Fatalf("行字段错: %+v", c)
	}
	if err := d.SaveFeishuCard(ctx, "t1", "om_2", "oc_1"); err != nil {
		t.Fatalf("save2: %v", err)
	}
	c, err = d.GetFeishuCard(ctx, "t1")
	if err != nil || c == nil || c.MessageID != "om_2" {
		t.Fatalf("同键覆盖未生效: c=%+v err=%v", c, err)
	}
}
