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
		fmt.Println("receive a pkt....")
	}, func() {
		client.Close()
		fmt.Printf("rtmp closed.")
	})
	if err != nil {
		panic(err)
	}

	select {}
}
