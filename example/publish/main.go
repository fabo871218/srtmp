package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/fabo871218/srtmp"
	"github.com/fabo871218/srtmp/av"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func PushH264(track *srtmp.StreamTrack) {
	data, err := ioutil.ReadFile("/Users/fabojiang/Desktop/video-files/test.h264")
	if err != nil {
		panic(err)
	}

	bcheck := true
	timeStamp := 0
	pre := 0
	index := 0
	for {
		if bytes.Compare(data[index:index+4], startCode) == 0 {
			if index > pre {
				if bcheck {
					preNaluType := data[pre+4] & 0x1F
					naluType := data[index+4] & 0x1F
					if preNaluType == 7 {
						if naluType != 7 && naluType != 8 {
							bcheck = false
						}
						index += 4
						continue
					}
				}

				bcheck = true

				msg := srtmp.StreamMessage{
					MessageType: srtmp.MessageTypeVideo,
					Pts:         uint32(timeStamp),
					Dts:         0,
					Payload:     make([]byte, index-pre),
				}
				copy(msg.Payload, data[pre:index])
				if err := track.WriteMessage(&msg); err != nil {
					panic(err)
				}
				time.Sleep(time.Millisecond * 40)
				timeStamp += 40

				pre = index
				index += 4
				continue
			}
		}
		index++
		if index+4 >= len(data) {
			index = 0
			pre = 0
		}
	}
}

func PushJPEG(client *srtmp.RtmpClient) {
	// index := 0
	// timeStamp := 0
	// for {
	// 	fileName := fmt.Sprintf("/Users/fabojiang/Documents/raw_jpeg_to_tuya/gc0308_img_%d.data", index)
	// 	data, err := ioutil.ReadFile(fileName)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	index++
	// 	if index >= 100 {
	// 		index = 0
	// 	}
	// 	pkt := &av.Packet{
	// 		PacketType: av.PacketTypeVideo,
	// 		TimeStamp:  uint32(timeStamp),
	// 		Data:       make([]byte, len(data)),
	// 		StreamID:   0,
	// 		VHeader: av.VideoPacketHeader{
	// 			FrameType:       av.FrameKey,
	// 			CodecID:         av.VideoJPEG,
	// 			CompositionTime: 0,
	// 		},
	// 	}
	// 	copy(pkt.Data, data)
	// 	if err := client.SendPacket(pkt); err != nil {
	// 		panic(err)
	// 	}
	// 	fmt.Println("Debug.... send jpeg.... ", len(data))
	// 	time.Sleep(time.Millisecond * 40)
	// 	timeStamp += 40
	// }
}

func main() {
	host := flag.String("host", "", "rtmp server host")
	port := flag.Int("port", 1935, "rtmp server port")
	flag.Parse()

	api := srtmp.NewAPI()
	client := api.NewRtmpClient()

	videoInfo := srtmp.VideoTrackInfo{
		CodecID: av.VideoH264,
		Width:   1920,
		Height:  1080,
	}

	track, err := client.AddStreamTrack(nil, &videoInfo)
	if err != nil {
		panic(err)
	}

	rtmpURL := fmt.Sprintf("rtmp://%s:%d/srtmp/livego", *host, *port)
	if err := client.OpenPublish(rtmpURL, func(state srtmp.RtmpConnectState, err error) {
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

	fmt.Println("Publish url:", rtmpURL)
	PushH264(track)
}
