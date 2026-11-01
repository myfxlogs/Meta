// Package config 网关配置加载（D-9：配置出码外，必载拒起）——
// config/gateway.textproto 经 prototext 解析成 GatewayConfig；
// 文件缺/坏＝fail loud（登记表在码里就不叫配置化）。
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/encoding/prototext"

	pbv1 "meta/internal/pb/metav1"
)

// DefaultPath 缺省配置路径：<可执行文件>/../config/gateway.textproto
// （生产布局 /opt/meta/config/——换装三件套同步落点）。
func DefaultPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "..", "config", "gateway.textproto")
}

// Load 读 textproto 成 GatewayConfig；缺/坏即错（错误带路径指引——
// serve 拒起时业主能直接照做）。
func Load(path string) (*pbv1.GatewayConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("网关配置必载（配置出码外）：读 %s 失败：%w——"+
			"请备 config/gateway.textproto 或以 --config 指定", path, err)
	}
	c := &pbv1.GatewayConfig{}
	if err := prototext.Unmarshal(b, c); err != nil {
		return nil, fmt.Errorf("网关配置解析 %s: %w", path, err)
	}
	return c, nil
}
