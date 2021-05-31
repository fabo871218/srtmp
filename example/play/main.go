package main

import (
	"encoding/binary"
	"flag"
	"fmt"

	"github.com/fabo871218/srtmp"
	"github.com/fabo871218/srtmp/logger"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func parseH264(data []byte) error {
	length := len(data)
	if length < 4 {
		return fmt.Errorf("bad length")
	}

	index := 0
	for length > 0 {
		naluLen := binary.BigEndian.Uint32(data[index : index+4])
		length = length - 4
		index += 4

		if naluLen > uint32(length) {
			return fmt.Errorf("bad nalu len, %d-%d", naluLen, length)
		}
		fmt.Println("Debug.... nalu len:", naluLen)
		length = length - int(naluLen)
		index += int(naluLen)
	}
	return nil
}

func main() {
	rtmpURL := flag.String("url", "", "rtmp play url")
	flag.Parse()

	api := srtmp.NewAPI(srtmp.WithLogLevel(logger.LogLevelDebug))
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
					fmt.Println("receive audio message...", len(msg.Payload))
				case srtmp.MessageTypeVideo:
					fmt.Println("receive video message....", len(msg.Payload))
					if err := parseH264(msg.Payload); err != nil {
						fmt.Println("parse video failed, ", err)
						//fmt.Println(hex.EncodeToString(msg.Payload[:]))
					}
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
