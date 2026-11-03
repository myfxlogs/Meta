// Package buildinfo version 输出去向（票0048-p1·决策100）：构建检出
// vcs 短哈希拼 stamp——go≥1.18 的 go build/go test 在 VCS 检出内自动
// 打 vcs.revision（-buildvcs 未关即默认），裸构建径零脚本改动；
// 戳缺席/取败→不拼 + 段退回 stamp 原文（无哈希降级不炸）。
// vcs.modified 脏标记刻意不入——bin/ 下被跟踪二进制部署换装必脏，
// 脏标恒真失辨识度（设计-0048 §4 弃案记档）。
package buildinfo

import "runtime/debug"

// Short 构建检出 vcs 短哈希：ReadBuildInfo→settings 扫 vcs.revision，
// 非空取前 7（与 git rev-parse --short 现行 7 位等长；短于 7 原文）。
// ReadBuildInfo 败/键缺/空串→""。
func Short() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return shortRev(bi.Settings)
}

// Version 拼 version 第二段：short 空→stamp 原文（降级无 +）；
// 非空→stamp+"+"+short。
func Version(stamp, short string) string {
	if short == "" {
		return stamp
	}
	return stamp + "+" + short
}

// shortRev vcs.revision 取前 7（取键逻辑拆纯函数缝供表测——对抗锚
// 提名行：键匹配行删则 Short() 恒 ""，实跑锚必红）。
func shortRev(settings []debug.BuildSetting) string {
	for _, s := range settings {
		if s.Key == "vcs.revision" {
			if len(s.Value) > 7 {
				return s.Value[:7]
			}
			return s.Value
		}
	}
	return ""
}
