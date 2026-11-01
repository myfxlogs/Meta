package clock

import (
	"testing"
	"time"
)

func waitFired(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s 未在限时内触发", what)
	}
}

// 一次性闹钟：到点响一次，不重复。
func TestAlarmFiresOnce(t *testing.T) {
	ch := make(chan struct{}, 4)
	NewAlarm(20*time.Millisecond, func() { ch <- struct{}{} })
	waitFired(t, ch, "闹钟")
	select {
	case <-ch:
		t.Fatal("一次性闹钟响了第二次")
	case <-time.After(100 * time.Millisecond):
	}
}

// 随事件重置：Reset 推后 deadline，原时刻不响。
func TestAlarmReset(t *testing.T) {
	ch := make(chan struct{}, 4)
	a := NewAlarm(80*time.Millisecond, func() { ch <- struct{}{} })
	after := time.AfterFunc(40*time.Millisecond, func() { a.Reset(400 * time.Millisecond) })
	defer after.Stop()
	// 原 deadline(80ms) 已过、新 deadline(440ms) 未至的窗口内不得响。
	select {
	case <-ch:
		t.Fatal("Reset 未生效：闹钟响在原 deadline")
	case <-time.After(150 * time.Millisecond):
	}
	waitFired(t, ch, "重置后的闹钟")
}

// 过期 deadline 立即响（启动对账重算径，§5.3 A4）。
func TestAlarmPastDeadlineFiresNow(t *testing.T) {
	ch := make(chan struct{}, 1)
	a := NewAlarmAt(time.Now().Add(-time.Hour), func() { ch <- struct{}{} })
	waitFired(t, ch, "过期闹钟")
	if !a.Fired() {
		t.Fatal("Fired 未置位")
	}
}

// Stop 取消未决闹钟（冻结/判死径）。
func TestAlarmStop(t *testing.T) {
	ch := make(chan struct{}, 1)
	a := NewAlarm(30*time.Millisecond, func() { ch <- struct{}{} })
	a.Stop()
	select {
	case <-ch:
		t.Fatal("Stop 后仍响")
	case <-time.After(100 * time.Millisecond):
	}
}
