package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/media/h264"
	"github.com/fabo871218/srtmp/protocol/amf"
)

func getflv() {
	response, err := http.Get("http://sf1-hscdn-tos.pstatp.com/obj/media-fe/xgplayer_doc_video/flv/xgplayer-demo-360p.flv")
	if err != nil {
		panic(err)
	}

	if response.StatusCode != 200 {
		panic(response.StatusCode)
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	w, err := os.OpenFile("/Users/fabojiang/Desktop/xigua.flv", os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		panic(err)
	}
	defer w.Close()

	w.Write(data)
}

func init() {
	http.HandleFunc("/flv", flvPlay)
}

func pushStream(w io.Writer) {
	data, err := ioutil.ReadFile("/Users/fabojiang/Desktop/video-files/test.h264")
	if err != nil {
		panic(err)
	}

	bfirst := true
	bcheck := true
	var timeStamp uint32 = 0
	pre := 0
	index := 0
	for {
		if index >= len(data)-4 {
			return
		}

		if bytes.Compare(data[index:index+4], []byte{0x00, 0x00, 0x00, 0x01}) == 0 {
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

				payload := data[pre:index]
				if bfirst {
					var sps, pps []byte
					nalus := h264.ParseNalus(payload)
					for _, nalu := range nalus {
						if naluType := nalu[0] & 0x1F; naluType == 7 {
							sps = nalu
						} else if naluType == 8 {
							pps = nalu
						}
					}

					if sps == nil || pps == nil {
						pre = index
						index += 4
						continue
					}

					sequenceData := flv.NewAVCSequenceHeader(sps, pps, timeStamp, true)
					if _, err := w.Write(sequenceData); err != nil {
						panic(err)
					}
					binary.Write(w, binary.BigEndian, uint32(len(sequenceData)))
					bfirst = false
				}

				vh := av.VideoPacketHeader{
					FrameType:       av.FrameInter,
					AVCPacketType:   av.AvcNALU,
					CodecID:         av.VideoH264,
					CompositionTime: 0,
				}
				videoData, err := flv.PackVideoData(&vh, true, 0, payload, timeStamp)
				if err != nil {
					panic(err)
				}
				fmt.Println("write video..... len:", len(videoData), hex.EncodeToString(videoData[:60]))

				if _, err := w.Write(videoData); err != nil {
					panic(err)
				}
				binary.Write(w, binary.BigEndian, uint32(len(videoData)))
				time.Sleep(time.Millisecond * 40)
				timeStamp += 40

				pre = index
				index += 4
				continue
			}
		}
		index++
	}
}

func flvPlay(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Receive new flv request....")

	w.Header().Set("Content-Type", "video/x-flv")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var flvHeader []byte
	flvHeader = append(flvHeader, []byte{'F', 'L', 'V'}...)
	flvHeader = append(flvHeader, []byte{0x01}...)
	flvHeader = append(flvHeader, []byte{0x05}...)
	flvHeader = append(flvHeader, []byte{0x00, 0x00, 0x00, 0x09}...)
	flvHeader = append(flvHeader, []byte{0x00, 0x00, 0x00, 0x00}...)

	if _, err := w.Write(flvHeader); err != nil {
		panic(err)
	}

	objmap := make(amf.Object)
	objmap["videocodecid"] = av.VideoH264
	objmap["height"] = 640
	objmap["width"] = 480

	bts := &bytes.Buffer{}
	encoder := amf.Encoder{}
	if _, err := encoder.Encode(bts, "onMetaData", amf.AMF0); err != nil {
		panic(err)
	}
	if _, err := encoder.Encode(bts, objmap, amf.AMF0); err != nil {
		panic(err)
	}

	metaData, err := flv.PackScriptData(av.TAG_SCRIPTDATAAMF0, 0, bts.Bytes())
	if err != nil {
		panic(err)
	}

	fmt.Println("Write meta data.... ", len(metaData))
	if _, err := w.Write(metaData); err != nil {
		panic(err)
	}
	binary.Write(w, binary.BigEndian, uint32(len(metaData)))
	fmt.Println("start to push stream....")
	pushStream(w)
}

func main() {
	http.ListenAndServe(":8081", nil)
}
