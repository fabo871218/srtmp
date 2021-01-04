package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/fabo871218/srtmp"
	"github.com/fabo871218/srtmp/av"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func handleVideoPacket(pkt *av.Packet) {
	switch pkt.VHeader.CodecID {
	case av.VIDEO_H264:
		fmt.Printf("receive h264 pkt... %s\n", hex.EncodeToString(pkt.Data[:4]))
		naluType := pkt.Data[0] & 0x1F
		switch naluType {
		case 7, 8, 1, 5:
			//file.Write([]byte{0x00, 0x00, 0x00, 0x01})
			//file.Write(pkt.Data)
		default:
			//忽略
		}
	case av.VIDEO_JPEG:
		fmt.Println("receive jpeg pkt... ", len(pkt.Data))
	default:
	}
}

func handleAudioPacket(pkt *av.Packet) {
	fmt.Printf("receive audio packet:%d\n", pkt.AHeader.SoundFormat)
}

func handleMetaPacket(pkt *av.Packet) {

}

func main() {
	rtmpURL := flag.String("url", "", "rtmp play url")
	flag.Parse()

	api := srtmp.NewAPI()
	client := api.NewRtmpClient()

	file, err := os.OpenFile("h.264", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	err = client.OpenPlay(*rtmpURL, func(pkt *av.Packet) {
		switch pkt.PacketType {
		case av.PacketTypeVideo:
			handleVideoPacket(pkt)
		case av.PacketTypeAudio:
			handleAudioPacket(pkt)
			//fmt.Printf("audio pkt, len:%d", len(pkt.Data))
		case av.PacketTypeMetadata:
			handleMetaPacket(pkt)
			// todo
		default:
		}
		//fmt.Println(len(pkt.Data))
	}, func() {
		fmt.Println("Closed...")
	})
	if err != nil {
		panic(err)
	}
	select {}
}
