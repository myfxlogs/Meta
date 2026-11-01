// Package seat 是真壳席位的外壳进程体（`meta seat`）：网关注入 run_token
// 与适配器参数后，本进程 Attach+Claim、起真壳 CLI 为子进程（前台 exec，
// B5 契约）、把壳 stdout 流经适配器转成 proto 帧上报、壳退出按分级码自退。
// per-seat UDS 收养口同步开（§5.1）；流断进收养等待，闹钟到自行退出。
package seat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"meta/internal/adapter"
	"meta/internal/inject"
	pbv1 "meta/internal/pb/metav1"
)

// 席位退出码约定（网关按码归失效分级；写进 seat_run.exit_code 可取证）。
const (
	ExitClean     = 0
	ExitSeatErr   = 2  // 外壳自身错（attach/claim/适配器构造失败）
	ExitOrphan    = 3  // 收养闹钟到未被收养（§5.1）
	ExitAbort     = 4  // 网关 Abort 令
	ExitSpawnFail = 61 // ① 起不来/秒退
	ExitTransient = 62 // ②a 限流/瞬态
	ExitFatal     = 63 // ②b 配额尽/授权/持续错
)

// Run 席位主程。全部参数经 env 注入（META_*，见 Spawn），返回进程退出码。
func Run() int {
	spec, err := specFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat:", err)
		return ExitSeatErr
	}
	s := &session{spec: spec, sendq: make(chan *pbv1.SeatMsg, 64),
		adopted: make(chan struct{}, 1), ackCh: make(chan struct{}, 1)}
	return s.run()
}

type spec struct {
	adapter.Spec
	GatewaySock string
	SeatSock    string
	RunToken    string
	TicketID    string
	RunID       string
	Interactive bool // META_INTERACTIVE=1：stdin 透传给壳（CEO 交互面）
	AdoptGrace  time.Duration
}

func specFromEnv() (spec, error) {
	e := os.Getenv
	s := spec{
		Spec: adapter.Spec{
			Shell:       e("META_SHELL"),
			Model:       e("META_MODEL"),
			Role:        e("META_ROLE"),
			PromptFile:  e("META_PROMPT_FILE"),
			WorkDir:     e("META_WORKDIR"),
			ArtifactDir: e("META_ART_DIR"),
		},
		GatewaySock: e("META_GATEWAY_SOCK"),
		SeatSock:    e("META_SEAT_SOCK"),
		RunToken:    e("META_RUN_TOKEN"),
		TicketID:    e("META_TICKET_ID"),
		RunID:       e("META_RUN_ID"),
		Interactive: e("META_INTERACTIVE") == "1",
		AdoptGrace:  600 * time.Second,
	}
	if g := e("META_ADOPT_GRACE"); g != "" {
		if sec, err := strconv.Atoi(g); err == nil {
			s.AdoptGrace = time.Duration(sec) * time.Second
		}
	}
	// META_TICKET_ID 空串仅 CEO run 合法（无头化身/交互面免 claim，票0011）。
	if s.Shell == "" || s.GatewaySock == "" || s.SeatSock == "" || s.RunToken == "" ||
		(s.TicketID == "" && s.Role != "CEO") {
		return s, fmt.Errorf("META_* 环境不全（shell=%q sock=%q seat=%q token=%v ticket=%q role=%q）",
			s.Shell, s.GatewaySock, s.SeatSock, s.RunToken != "", s.TicketID, s.Role)
	}
	return s, nil
}

// session 一次席位运行。
type session struct {
	spec    spec
	sendq   chan *pbv1.SeatMsg
	adopted chan struct{}

	mu           sync.Mutex
	stream       pbv1.SeatGateway_AttachClient
	streamDead   bool
	finishing    bool // Deliver 已发：流死不再走收养等
	ackCh        chan struct{}
	textSeen     int64
	tail         string // 壳 stderr/stdout 尾，分级输入
	tailLastUsage int64 // 流内已报用量（终值补差基准）
	resultOK     bool   // 见到 FrameResult
	verdict      string // META_VERDICT 哨兵捕到的审计判决（pass|reject）
	verdictRef   string // 判决档 ref
	deliverRef   string // META_DELIVER 哨兵捕到的交付 ref（D-32）
	deliverSeen  bool   // META_DELIVER 哨兵已见（净退无哨兵→ExitSeatErr）
}

// adapterNew 注入缝（测试换伪适配器——哨兵闸的壳行为由 argv 脚本驱动）。
var adapterNew = adapter.New

// run 主流程：收养口 → attach+claim → 起壳 → 帧泵 → 退码分级自退。
func (s *session) run() int {
	ad, err := adapterNew(s.spec.Shell)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat:", err)
		return ExitSeatErr
	}
	srv, err := s.serveAdopt()
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat: 收养口:", err)
		return ExitSeatErr
	}
	defer srv.Stop()

	if err := s.attach(); err != nil {
		fmt.Fprintln(os.Stderr, "seat: attach:", err)
		return ExitSeatErr
	}
	// META_TICKET_ID 空＝CEO run：免 claim（无票可领，§1.2）。
	if s.spec.TicketID != "" {
		if err := s.enqueue(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{
			Claim: &pbv1.Claim{TicketId: s.spec.TicketID}}}); err != nil {
			return ExitSeatErr
		}
	}

	code, tail := s.runChild(ad)
	class := ad.Classify(code, tail)
	if tok, ok := ad.FinalTokens(s.spec.Spec); ok {
		s.sendUsageFinal(tok)
	} else {
		s.enqueue(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Progress{Progress: &pbv1.Progress{
			Stage: "交付", Note: "用量不可得（壳流/产物均无）——如实标，未估算"}}})
	}
	// 发件队列落尽再退——Deliver/Usage 不能丢在 buffer 里。
	s.drain()
	switch class {
	case adapter.ExitOK:
		s.mu.Lock()
		s.finishing = true
		s.mu.Unlock()
		// 无票 run（CEO 面）不送 Deliver——无相位可翻。
		if s.spec.TicketID == "" {
			return ExitClean
		}
		// D-32 机械哨兵闸（令#E15 首个适用）：净退无哨兵＝席死径
		// （ExitSeatErr＋不发 Deliver——完工没喊交货就当没交）。审计席的
		// 交付哨兵＝META_VERDICT（携判决+ref）；其余角色＝META_DELIVER:<ref>。
		s.mu.Lock()
		verdict, vref, dref, dseen := s.verdict, s.verdictRef, s.deliverRef, s.deliverSeen
		s.mu.Unlock()
		if s.spec.Role == "审计" {
			if verdict == "" {
				fmt.Fprintln(os.Stderr, "seat: 净退无 META_VERDICT 哨兵——不交付")
				return ExitSeatErr
			}
		} else if !dseen {
			fmt.Fprintln(os.Stderr, "seat: 净退无 META_DELIVER 哨兵——不交付")
			return ExitSeatErr
		}
		ref := dref
		if s.spec.Role == "审计" {
			ref = vref
		}
		s.enqueueSync(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Deliver{Deliver: &pbv1.Deliver{
			Ref: ref, Verdict: verdict,
			Note: fmt.Sprintf("%s exit=0%s", s.spec.Shell, resultNote(s.resultOK))}}})
		// 等 Deliver 回执（有界 3s）——尾帧不许丢在传输缓冲里。
		select {
		case <-s.ackCh:
		case <-time.After(3 * time.Second):
		}
		return ExitClean
	}
	return ExitCodeOf(class) // 非 OK 分级→席位退码：与网关 ClassOfExit 同表
}

func resultNote(ok bool) string {
	if ok {
		return "（有 result 帧）"
	}
	return ""
}

// runChild 起壳子进程（前台 exec——不 daemonize，B5）；泵 stdout 帧；
// 返回退码与输出尾。子进程与本席同进程组，网关爱杀全组。
func (s *session) runChild(ad adapter.Adapter) (int, string) {
	c, err := ad.Build(s.spec.Spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat: build:", err)
		return -1, err.Error()
	}
	cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
	cmd.Dir = c.Dir
	cmd.Env = append(os.Environ(), c.Env...)
	if s.spec.Interactive {
		cmd.Stdin = os.Stdin // META_INTERACTIVE=1：业主打字直透壳（CEO 交互面）
	} else if c.StdinFile != "" {
		f, err := os.Open(c.StdinFile)
		if err != nil {
			return -1, "prompt file: " + err.Error()
		}
		defer f.Close()
		cmd.Stdin = f
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err.Error()
	}
	// stderr 合并进尾缓冲（分级输入）同时透传到席位日志。
	cmd.Stderr = io.MultiWriter(os.Stderr, &tailWriter{s: s})
	if err := cmd.Start(); err != nil {
		return -1, "spawn: " + err.Error() // ① 起不来
	}

	// 网关消息泵：Abort/PermissionAsk；流死走收养等待。
	go s.pumpGateway()

	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var lastUsage int64
	for sc.Scan() {
		l := sc.Bytes()
		s.noteTail(l)
		os.Stdout.Write(append(l, '\n')) // 原文留痕进席位日志
		if s.scanSentinel(l) {
			continue // 哨兵行不进适配器（壳与外壳的约定，与壳无关）
		}
		for _, fr := range ad.Parse(l) {
			switch fr.Kind {
			case adapter.FrameText:
				s.textSeen++
				if s.textSeen%32 == 1 { // 活性帧稀释：每 32 行透一帧，进度闹钟照活
					s.sendProgress(fr.Stage, fr.Note)
				}
			case adapter.FrameUsage:
				if d := fr.Tokens - lastUsage; d > 0 {
					lastUsage = fr.Tokens
					s.sendUsageDelta(d)
				}
			case adapter.FrameResult:
				s.resultOK = true
				if d := fr.Tokens - lastUsage; d > 0 {
					lastUsage = fr.Tokens
					s.sendUsageDelta(d)
				}
				s.sendProgress("交付", fr.Note)
			case adapter.FrameError:
				s.sendProgress("错误", fr.Note)
			}
		}
	}
	werr := cmd.Wait()
	s.mu.Lock()
	s.tailLastUsage = lastUsage
	s.mu.Unlock()
	code := -1
	if ee, ok := werr.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if werr == nil {
		code = 0
	}
	return code, s.tailSnapshot()
}

// pumpGateway 守流收 GatewayMsg；EOF/错误＝socket 源断（§5.2 双源之一）。
// 断流后装一次性收养闹钟：收养＝重 attach 续跑，闹钟到＝自行退出（§5.1）。
func (s *session) pumpGateway() {
	s.mu.Lock()
	stream := s.stream
	s.mu.Unlock()
	if stream == nil {
		return
	}
	for {
		msg, err := stream.Recv()
		if err != nil {
			s.mu.Lock()
			s.streamDead = true
			fin := s.finishing
			s.mu.Unlock()
			if !fin {
				s.waitAdopt()
			}
			return
		}
		switch b := msg.GetBody().(type) {
		case *pbv1.GatewayMsg_Ack:
			select {
			case s.ackCh <- struct{}{}:
			default:
			}
		case *pbv1.GatewayMsg_AdminResp:
			fmt.Fprintf(os.Stderr, "seat: admin_resp ok=%v %s\n",
				b.AdminResp.Ok, b.AdminResp.Note)
		case *pbv1.GatewayMsg_Abort:
			fmt.Fprintln(os.Stderr, "seat: Abort:", b.Abort.Reason)
			os.Exit(ExitAbort)
		case *pbv1.GatewayMsg_PermissionAsk:
			// 许可提示自动答拒（禁区#12：席位不等人）。
			s.enqueue(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_PermissionReply{
				PermissionReply: &pbv1.PermissionReply{AskId: b.PermissionAsk.AskId, Allow: false}}})
		}
	}
}

// waitAdopt 流断后等收养：一次性闹钟（AfterFunc）；被收养＝重 attach；
// 闹钟到＝ExitOrphan。席位禁无限自行重连（§5.1）。
func (s *session) waitAdopt() {
	quit := make(chan struct{})
	t := time.AfterFunc(s.spec.AdoptGrace, func() { close(quit) })
	defer t.Stop()
	select {
	case <-s.adopted:
		if err := s.attach(); err == nil {
			s.mu.Lock()
			s.streamDead = false
			s.mu.Unlock()
			_ = s.enqueue(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Claim{
				Claim: &pbv1.Claim{TicketId: s.spec.TicketID}}}) // 重领幂等
			go s.pumpGateway()
			return
		}
		os.Exit(ExitOrphan)
	case <-quit:
		os.Exit(ExitOrphan)
	}
}

// attach 连主 socket→RunAuth 首帧→挂发件泵。有界重试链（3 次，退避递增）。
func (s *session) attach() error {
	var last error
	for i := 0; i < 3; i++ {
		if i > 0 {
			<-time.After(time.Duration(i) * 500 * time.Millisecond) // 有界退避
		}
		if err := s.attachOnce(); err == nil {
			return nil
		} else {
			last = err
		}
	}
	return last
}

func (s *session) attachOnce() error {
	conn, err := grpc.NewClient("unix://"+s.spec.GatewaySock,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	stream, err := pbv1.NewSeatGatewayClient(conn).Attach(context.Background())
	if err != nil {
		conn.Close()
		return err
	}
	if err := stream.Send(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Auth{
		Auth: &pbv1.RunAuth{RunToken: s.spec.RunToken}}}); err != nil {
		conn.Close()
		return err
	}
	s.mu.Lock()
	s.stream = stream
	s.mu.Unlock()
	go s.sendPump(stream, conn)
	return nil
}

// sendPump 唯一发件者（gRPC 流并发 Send 不安全，一切外发经 sendq）。
func (s *session) sendPump(stream pbv1.SeatGateway_AttachClient, conn *grpc.ClientConn) {
	for m := range s.sendq {
		if err := stream.Send(m); err != nil {
			return
		}
	}
	_ = conn.Close
}

// enqueue 进外发队列；满则丢活性帧不丢语义帧（Progress 阶段帧/Usage/Deliver 直发）。
func (s *session) enqueue(m *pbv1.SeatMsg) error {
	select {
	case s.sendq <- m:
	default:
		if st := s.direct(); st != nil {
			return st.Send(m) // 队列满→同步直发，宁阻塞不丢语义帧
		}
		return fmt.Errorf("seat: 流未立")
	}
	return nil
}

// enqueueSync 收尾帧直发（Drain 阶段泵已停）。
func (s *session) enqueueSync(m *pbv1.SeatMsg) {
	if st := s.direct(); st != nil {
		_ = st.Send(m)
	}
}

func (s *session) direct() pbv1.SeatGateway_AttachClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.streamDead {
		return nil
	}
	return s.stream
}

// drain 给发件泵落尽窗口——有界等待队列排空（AfterFunc 一次性，非轮询）。
func (s *session) drain() {
	deadline := time.Now().Add(3 * time.Second)
	for len(s.sendq) > 0 && time.Now().Before(deadline) {
		<-time.After(5 * time.Millisecond)
	}
}

// scanSentinel 壳 stdout 哨兵扫描（壳与外壳的约定行，不进 adapter——
// 裁决席 META_RULE/审计席 META_VERDICT 均「最后单独一行」钦定，§1.4/§15.2）。
// 命中即消费返回 true。
func (s *session) scanSentinel(l []byte) bool {
	line := strings.TrimSpace(string(l))
	if rest, ok := strings.CutPrefix(line, "META_RULE:"); ok {
		action, note, _ := strings.Cut(rest, ":")
		s.enqueue(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Rule{
			Rule: &pbv1.Rule{Action: action, Note: note}}})
		return true
	}
	if rest, ok := strings.CutPrefix(line, "META_VERDICT:"); ok {
		v, ref, _ := strings.Cut(rest, ":")
		s.mu.Lock()
		s.verdict, s.verdictRef = v, ref
		s.mu.Unlock()
		return true
	}
	if rest, ok := strings.CutPrefix(line, "META_DELIVER:"); ok {
		// D-32 交付哨兵（令#E15）：净退无此行＝席死径不交付。
		s.mu.Lock()
		s.deliverRef, s.deliverSeen = rest, true
		s.mu.Unlock()
		return true
	}
	if rest, ok := strings.CutPrefix(line, "META_ADMIN:"); ok {
		return s.adminSentinel(rest)
	}
	return false
}

// adminSentinel META_ADMIN:<动作>|<args> → SeatAdmin 帧（§1.5：CTO-A 写径
// 随席位流走）。fill_spec/create_ticket 的 spec 均为票面 markdown 文件路径。
func (s *session) adminSentinel(rest string) bool {
	parts := strings.SplitN(rest, "|", 4)
	send := func(a *pbv1.SeatAdmin) {
		s.enqueue(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Admin{Admin: a}})
	}
	switch parts[0] {
	case "fill_spec":
		if len(parts) < 2 {
			return false
		}
		b, err := os.ReadFile(parts[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "seat: fill_spec 读文件:", err)
			return true
		}
		send(&pbv1.SeatAdmin{Action: &pbv1.SeatAdmin_FillSpec{
			FillSpec: &pbv1.FillSpec{TicketId: s.spec.TicketID, SpecMd: b}}})
	case "create_ticket":
		// META_ADMIN:create_ticket|<id>|<title>|<spec.md>
		if len(parts) < 4 {
			return false
		}
		nt := &pbv1.NewTicket{Id: parts[1], Title: parts[2]}
		if b, err := os.ReadFile(parts[3]); err == nil {
			if ts, err := inject.ParseTicketMD(b); err == nil {
				nt.Spec, _ = proto.Marshal(ts)
			}
		}
		send(&pbv1.SeatAdmin{Action: &pbv1.SeatAdmin_CreateTicket{CreateTicket: nt}})
	case "dispatch":
		// META_ADMIN:dispatch|<ticket>|<role>|[promptfile]|[workdir]
		if len(parts) < 3 {
			return false
		}
		d := &pbv1.DispatchReq{TicketId: parts[1], Role: parts[2]}
		if len(parts) > 3 && parts[3] != "" {
			d.PromptFile, d.WorkDir, _ = strings.Cut(parts[3], "~")
		}
		send(&pbv1.SeatAdmin{Action: &pbv1.SeatAdmin_Dispatch{Dispatch: d}})
	default:
		return false
	}
	return true
}

func (s *session) sendProgress(stage, note string) {
	s.enqueue(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Progress{
		Progress: &pbv1.Progress{Stage: stage, Note: note}}})
}

func (s *session) sendUsageDelta(d int64) {
	s.enqueue(&pbv1.SeatMsg{Body: &pbv1.SeatMsg_Usage{
		Usage: &pbv1.Usage{TokensUsed: d}}})
}

// sendUsageFinal 终值口径补差：流内已报过就补差值，没报过全报。
func (s *session) sendUsageFinal(total int64) {
	s.mu.Lock()
	last := s.tailLastUsage
	s.mu.Unlock()
	if d := total - last; d > 0 {
		s.sendUsageDelta(d)
	}
}

func (s *session) noteTail(l []byte) {
	s.mu.Lock()
	s.tail = tailKeep(s.tail + string(l) + "\n")
	s.mu.Unlock()
}

func (s *session) tailSnapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tail
}

// serveAdopt per-seat UDS 收养口（0600）：网关出示 run_token，验不过拒。
func (s *session) serveAdopt() (*grpc.Server, error) {
	_ = os.Remove(s.spec.SeatSock)
	ln, err := net.Listen("unix", s.spec.SeatSock)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(s.spec.SeatSock, 0o600)
	srv := grpc.NewServer()
	pbv1.RegisterSeatAdoptServer(srv, &adoptSvc{token: s.spec.RunToken,
		ticketID: s.spec.TicketID, accepted: s.adopted})
	go srv.Serve(ln)
	return srv, nil
}

// adoptSvc 收养互证（§5.1 D2）：它发的它知道。
type adoptSvc struct {
	pbv1.UnimplementedSeatAdoptServer
	token    string
	ticketID string
	accepted chan struct{}
}

func (a *adoptSvc) Adopt(_ context.Context, au *pbv1.AdoptAuth) (*pbv1.AdoptAck, error) {
	if au.RunToken != a.token {
		return nil, status.Error(codes.PermissionDenied, "run_token 不符")
	}
	select {
	case a.accepted <- struct{}{}:
	default:
	}
	return &pbv1.AdoptAck{Accepted: true, TicketId: a.ticketID}, nil
}

// tailWriter 壳 stderr 透传＋留尾（分级输入）。
type tailWriter struct{ s *session }

func (w *tailWriter) Write(p []byte) (int, error) {
	w.s.noteTail(p)
	return len(p), nil
}

func tailKeep(s string) string {
	if len(s) > 4096 {
		return s[len(s)-4096:]
	}
	return s
}

// ExitCodeOf 壳退码→席位退出码映射（与 ClassOfExit 同表互逆，单一映射源）。
func ExitCodeOf(class adapter.ExitClass) int {
	switch class {
	case adapter.ExitTransient:
		return ExitTransient
	case adapter.ExitFatal:
		return ExitFatal
	case adapter.ExitOK:
		return ExitClean
	}
	return ExitSpawnFail
}

// ClassOfExit 席位退码→失效分级（网关侧用）。
func ClassOfExit(code int) adapter.ExitClass {
	switch code {
	case ExitTransient:
		return adapter.ExitTransient
	case ExitFatal:
		return adapter.ExitFatal
	case ExitClean:
		return adapter.ExitOK
	}
	return adapter.ExitSpawn
}
