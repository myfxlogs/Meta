package gateway

// Event 网关出口事件（§3.3：事件就是调用本身；此处是进程内观察口，
// 持久日志在 ticket_event）。Kind 取值：attach / spawn_needed / adopted /
// adopt_failed / seat_dead / requeue / stall_suspect / phase:<EdgeKind>。
type Event struct {
	Kind     string
	TicketID string
	RunID    string
	Role     string
	Note     string
}
