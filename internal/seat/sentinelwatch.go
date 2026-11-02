package seat

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// watchSentinel 哨兵文件泵第三触发径（票0045·设计-0034-p3-p1 §2）：
// inotify 盯 $META_ART_DIR 目录的 sentinel.log 名事件→泵——壳 stdout
// 静默期（devin -p 块缓冲）运行中动作哨兵（META_EXEC 等）仍被泵走，
// 与 :317/:320 两既有泵点并存、语义不动。
// cmd.Start() 成功后起；返回 stop（关停管→watcher 归队，供 :320 终泵
// 压轴前收口）。ArtifactDir 空/init/addwatch/pipe 任一步败→不起
// watcher＋stderr 留痕＋nil——降级不劣于现状。
func (s *session) watchSentinel() func() {
	dir := s.spec.ArtifactDir
	if dir == "" {
		return nil
	}
	ifd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seat: sentinel watch init:", err)
		return nil
	}
	if _, err := syscall.InotifyAddWatch(ifd, dir, syscall.IN_MODIFY|
		syscall.IN_CLOSE_WRITE|syscall.IN_CREATE|syscall.IN_MOVED_TO); err != nil {
		fmt.Fprintln(os.Stderr, "seat: sentinel watch add:", err)
		syscall.Close(ifd)
		return nil
	}
	var p [2]int
	if err := syscall.Pipe2(p[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		fmt.Fprintln(os.Stderr, "seat: sentinel watch pipe:", err)
		syscall.Close(ifd)
		return nil
	}
	done := make(chan struct{})
	go s.sentinelWatch(ifd, p[0], done)
	return func() {
		syscall.Close(p[1]) // 写端关→停管 EOF 恒定可读→watcher 确定性归队
		<-done
		syscall.Close(p[0])
		syscall.Close(ifd)
	}
}

// sentinelWatch watcher 协程体：inotify-fd＋停管二元 Select（nil timeout
// 无限阻塞，零定时器——内核文件事件即唯一唤醒源）。ifd 可读→非阻塞排干
// 事件批→一次泵；stopR 可读→净退。起机先泵一次存量：AddWatch 前落盘
// 行无事件可发，补捕起机窗口。
func (s *session) sentinelWatch(ifd, stopR int, done chan<- struct{}) {
	defer close(done)
	var buf [8192]byte
	s.pumpSentinelFile() // 起机排干存量——watch 立前落盘行补捕
	for {
		var rset syscall.FdSet
		fdSet(&rset, ifd)
		fdSet(&rset, stopR)
		nfd := ifd
		if stopR > nfd {
			nfd = stopR
		}
		if _, err := syscall.Select(nfd+1, &rset, nil, nil, nil); err != nil {
			if err == syscall.EINTR {
				continue
			}
			fmt.Fprintln(os.Stderr, "seat: sentinel watch select:", err)
			return
		}
		if fdIsSet(&rset, stopR) {
			return
		}
		if !fdIsSet(&rset, ifd) {
			continue
		}
		need, dead := drainInotifyEvents(ifd, buf[:])
		if dead {
			return // IN_IGNORED：watch 被摘/目录消失——净退
		}
		if need {
			s.pumpSentinelFile() // 每事件批一次——泵体 offset 幂等，空泵零成本
		}
	}
}

// drainInotifyEvents 非阻塞排干一批 inotify 事件（IN_NONBLOCK→EAGAIN 批尽）。
// sentinel.log 名命中或 IN_Q_OVERFLOW（溢队列可能漏行，保守补泵）→need；
// IN_IGNORED→dead。事件头四位组手解——免 unsafe 指针对齐险（checkptr）。
func drainInotifyEvents(ifd int, buf []byte) (need, dead bool) {
	for {
		n, err := syscall.Read(ifd, buf)
		if err != nil || n <= 0 {
			return need, dead
		}
		for off := 0; off+syscall.SizeofInotifyEvent <= n; {
			mask := binary.LittleEndian.Uint32(buf[off+4:])
			nlen := int(binary.LittleEndian.Uint32(buf[off+12:]))
			off += syscall.SizeofInotifyEvent
			switch {
			case mask&syscall.IN_IGNORED != 0:
				dead = true
			case mask&syscall.IN_Q_OVERFLOW != 0:
				need = true
			case nlen > 0 && off+nlen <= n &&
				filepath.Base(strings.TrimRight(string(buf[off:off+nlen]), "\x00")) == "sentinel.log":
				need = true
			}
			off += nlen
		}
	}
}

// fdSet/fdIsSet syscall.FdSet 位操作——stdlib syscall 无 FD_* 宏面，
// 又不引 x/sys 第三方（票0045 钦定纯 stdlib）。
func fdSet(set *syscall.FdSet, fd int) {
	set.Bits[fd/64] |= 1 << uint(fd%64)
}

func fdIsSet(set *syscall.FdSet, fd int) bool {
	return set.Bits[fd/64]&(1<<uint(fd%64)) != 0
}
