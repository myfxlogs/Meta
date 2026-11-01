package gateway

import (
	"bytes"
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"

	"meta/internal/db"
)

// BoardTokenName secret 表行名——`meta secret put secret.board.token` 落种
// （业主凭证动作；行缺＝前端面一律 Unauthenticated，fail closed）。
const BoardTokenName = "secret.board.token"

// GRPCServer 暴露底层 gRPC server——Board 在同一 unix socket 上追加注册
// （meta board --socket 验收径；board_listen TCP 面另挂 Connect 二进制）。
func (g *Gateway) GRPCServer() *grpc.Server { return g.srv }

// Subscribe 进程内事件订户口（与 Observe 面同源；板每连一订）。
func (g *Gateway) Subscribe() chan Event { return g.subscribe() }

// Unsubscribe 除名订户（流断即退）。
func (g *Gateway) Unsubscribe(ch chan Event) { g.unsubscribe(ch) }

// CheckBoardToken 前端面 token 闸（§3.4 loopback+token 最小暴露面）：
// vault.key 缺载 / secret.board.token 行缺 / 解密失败 → false（fail closed）。
// 对抗性锚提名行：等值比较——翻成 !=/恒真则错 token 也放行，锚红。
func (g *Gateway) CheckBoardToken(ctx context.Context, token string) bool {
	if token == "" || g.vaultKey == nil {
		return false
	}
	ct, ok, err := g.database.SecretBlob(ctx, BoardTokenName)
	if err != nil || !ok {
		return false
	}
	plain, err := db.DecryptSecret(g.vaultKey, ct)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), bytes.TrimSpace(plain)) == 1
}
