// 一次性审计 harness（用后即删）：直调 inject.ParseTicketMD 复算锚 2/5，
// 并解 DB spec blob 对口径。零仓面改动。
package main

import (
	"fmt"
	"os"

	"meta/internal/inject"
	pbv1 "meta/internal/pb/metav1"

	"google.golang.org/protobuf/proto"
)

func lens(s *pbv1.TicketSpec) string {
	return fmt.Sprintf("goal=%dB why=%dB scope=%dB oos=%dB refs=%dB anchors=%dB verify=%dB deliver=%dB appendix=%dB verdicts=%dB",
		len(s.GetGoal()), len(s.GetWhy()), len(s.GetScope()), len(s.GetOutOfScope()),
		len(s.GetRefs()), len(s.GetAnchors()), len(s.GetVerify()), len(s.GetDeliver()),
		len(s.GetAppendix()), len(s.GetVerdicts()))
}

func main() {
	for _, p := range os.Args[1:] {
		if len(p) > 0 && p[0] == '@' { // DB blob 文件
			b, err := os.ReadFile(p[1:])
			if err != nil {
				fmt.Println("BLOB_READ_ERR", p, err)
				continue
			}
			ts := &pbv1.TicketSpec{}
			if err := proto.Unmarshal(b, ts); err != nil {
				fmt.Println("BLOB_UNMARSHAL_ERR", p, err)
				continue
			}
			fmt.Printf("BLOB_OK %s bloblen=%dB %s\n", p[1:], len(b), lens(ts))
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Println("MD_READ_ERR", p, err)
			continue
		}
		ts, err := inject.ParseTicketMD(b)
		if err != nil {
			fmt.Printf("PARSE_FAIL %s: %v\n", p, err)
			continue
		}
		blob, err := proto.Marshal(ts)
		if err != nil {
			fmt.Println("MARSHAL_ERR", p, err)
			continue
		}
		fmt.Printf("PARSE_OK %s md=%dB marshaled_blob=%dB %s\n", p, len(b), len(blob), lens(ts))
	}
}
