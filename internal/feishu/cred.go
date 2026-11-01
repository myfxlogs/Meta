package feishu

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	pbv1 "meta/internal/pb/metav1"
)

// ParseCred secret.feishu.app 解密明文→FeishuApp（格式核验双试）：
// FeishuApp proto blob 先试，KEY=VALUE 行格式兜底（迁移期 env 原文直存，
// FEISHU_MANAGE_* 全名与裸名都收）。必填四键缺一即错——调用方按错发
// feishu_cred_bad 事件并「通道不启」（fail loud，带病通道比没通道毒）。
func ParseCred(plain []byte) (*pbv1.FeishuApp, error) {
	app := &pbv1.FeishuApp{}
	if err := proto.Unmarshal(plain, app); err == nil && credComplete(app) {
		return app, nil
	}
	kv := map[string]string{}
	for _, line := range strings.Split(string(plain), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	pick := func(names ...string) string {
		for _, n := range names {
			if kv[n] != "" {
				return kv[n]
			}
		}
		return ""
	}
	app = &pbv1.FeishuApp{
		AppId:       pick("FEISHU_MANAGE_APP_ID", "APP_ID"),
		AppSecret:   pick("FEISHU_MANAGE_APP_SECRET", "APP_SECRET"),
		OwnerOpenId: pick("FEISHU_MANAGE_OWNER_OPEN_ID", "OWNER_OPEN_ID"),
		ChatId:      pick("FEISHU_MANAGE_CHAT_ID", "CHAT_ID"),
	}
	if !credComplete(app) {
		return nil, fmt.Errorf("凭证必填键不齐（APP_ID/APP_SECRET/OWNER_OPEN_ID/CHAT_ID）")
	}
	return app, nil
}

func credComplete(a *pbv1.FeishuApp) bool {
	return a.AppId != "" && a.AppSecret != "" && a.OwnerOpenId != "" && a.ChatId != ""
}
