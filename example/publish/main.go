package main

import (
	"bytes"
	"io/ioutil"
	"time"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/protocol"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func main() {
	client := protocol.NewRtmpClient()
	if err := client.OpenPublish("rtmp://127.0.0.1:1935/srtmp/livego"); err != nil {
		panic(err)
	}

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
				pkt := &av.Packet{
					IsVideo:   true,
					TimeStamp: uint32(timeStamp),
					Data:      make([]byte, index-pre),
				}
				copy(pkt.Data, data[pre:index])
				if err := client.SendPacket(pkt); err != nil {
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
			break
		}
	}
}
