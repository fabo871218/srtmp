package main

import (
	"os"

	"github.com/fabo871218/srtmp"
	"github.com/fabo871218/srtmp/av"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func main() {
	api := srtmp.NewAPI()
	client := api.NewRtmpClient()

	file, err := os.OpenFile("h.264", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	rtmpURL := "rtmp://58.200.131.2:1935/livetv/dftv"
	err = client.OpenPlay(rtmpURL, func(pkt *av.Packet) {
		if pkt.IsVideo {
			naluType := pkt.Data[0] & 0x1F
			switch naluType {
			case 7, 8, 1, 5:
				file.Write([]byte{0x00, 0x00, 0x00, 0x01})
				file.Write(pkt.Data)
			default:
				//忽略
			}
		} else if pkt.IsAudio {
			//fmt.Printf("audio pkt, len:%d", len(pkt.Data))
		}
		//fmt.Println(len(pkt.Data))
	}, func() {
		//fmt.Println("Closed...")
	})
	if err != nil {
		panic(err)
	}
	select {}
}
