// Package clock 实现「随事件重置的一次性闹钟」——总设计 R2 唯一允许的时钟。
// 内部用 time.AfterFunc；无 Ticker、无轮询、无周期任务。
package clock

import (
	"sync"
	"time"
)

// Alarm 一次性闹钟：到点发一次回调；Reset 随事件推后；ResetAt 支持把
// deadline 设到过去（立即响——启动对账「已过期即报」的落点，§5.3 A4）。
type Alarm struct {
	mu       sync.Mutex
	timer    *time.Timer
	deadline time.Time
	onFire   func()
	fired    bool
	stopped  bool
	now      func() time.Time
}

// NewAlarm 以相对时长起闹钟。d<=0 立即响。
func NewAlarm(d time.Duration, onFire func()) *Alarm {
	return NewAlarmAt(time.Now().Add(d), onFire)
}

// NewAlarmAt 以绝对 deadline 起闹钟。deadline 已过＝立即响。
func NewAlarmAt(deadline time.Time, onFire func()) *Alarm {
	a := &Alarm{deadline: deadline, onFire: onFire, now: time.Now}
	a.arm(deadline)
	return a
}

// Reset 随事件重置：从 now 起再算 d。
func (a *Alarm) Reset(d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.armLocked(a.now().Add(d))
}

// ResetAt 重置到指定 deadline（重启自 DB 重算用）。
func (a *Alarm) ResetAt(deadline time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.armLocked(deadline)
}

// Deadline 当前 deadline。
func (a *Alarm) Deadline() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.deadline
}

// Fired 是否已响过。
func (a *Alarm) Fired() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.fired
}

// Stop 停闹钟（票冻结/交付/判死时取消未决闹钟，§5.5 D8）。
func (a *Alarm) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopped = true
	if a.timer != nil {
		a.timer.Stop()
	}
}

func (a *Alarm) arm(deadline time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.armLocked(deadline)
}

func (a *Alarm) armLocked(deadline time.Time) {
	if a.stopped {
		return
	}
	a.deadline = deadline
	a.fired = false
	if a.timer != nil {
		a.timer.Stop()
	}
	d := deadline.Sub(a.now())
	if d < 0 {
		d = 0 // 已过期：立即响
	}
	a.timer = time.AfterFunc(d, a.fire)
}

func (a *Alarm) fire() {
	a.mu.Lock()
	if a.stopped || a.fired {
		a.mu.Unlock()
		return
	}
	a.fired = true
	cb := a.onFire
	a.mu.Unlock()
	if cb != nil {
		cb()
	}
}
