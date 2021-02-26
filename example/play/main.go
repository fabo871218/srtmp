package main

import (
	"flag"
	"fmt"

	"github.com/fabo871218/srtmp"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func main() {
	rtmpURL := flag.String("url", "", "rtmp play url")
	flag.Parse()

	api := srtmp.NewAPI()
	client := api.NewRtmpClient()

	if err := client.OpenPlay(*rtmpURL, func(state srtmp.RtmpConnectState, err error) {
		switch state {
		case srtmp.StateConnectSuccess:
			fmt.Println("connect success......")
		case srtmp.StateConnectFailed:
			fmt.Println("connect failed.....")
		case srtmp.StateDisconnect:
			client.Close()
			fmt.Println("Debug.... disconnect....")
		}
	}); err != nil {
		panic(err)
	}

	client.OnStreamTrack(func(track *srtmp.StreamTrack) {
		fmt.Println("Receive a new track..... ")
		go func() {
			for {
				msg, err := track.ReadMessage()
				if err != nil {
					panic(err)
				}

				switch msg.MessageType {
				case srtmp.MessageTypeAudio:
					fmt.Println("receive audio message...")
				case srtmp.MessageTypeVideo:
					fmt.Println("receive video message....", track.VideoInfo().CodecID, track.VideoInfo().Height, track.VideoInfo().Width)
				case srtmp.MessageTypeMateData:
					fmt.Println("receive mate data message....")
				default:
					fmt.Println("unknown message type,", msg.MessageType)
				}
			}
		}()
	})

	select {}
}
