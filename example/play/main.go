package main

import (
	"fmt"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/protocol"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func main() {
	client := protocol.NewRtmpClient()
	err := client.OpenPlay("rtmp://p2p.tuyacn.com:1935/srtmp/test", func(pkt *av.Packet) {
		if pkt.IsVideo {
			fmt.Printf("Receive a video, timestamp:%d\n", pkt.TimeStamp)
		} else if pkt.IsAudio {
			fmt.Printf("Receive a audio, timestamp:%d\n", pkt.TimeStamp)
		}
	}, func() {
		client.Close()
		fmt.Printf("rtmp closed.")
	})
	if err != nil {
		panic(err)
	}

	select {}
}
