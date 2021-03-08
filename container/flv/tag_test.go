package flv

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"os"
	"testing"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/media/h264"
	"github.com/fabo871218/srtmp/protocol/amf"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func TestFlvPack(t *testing.T) {
	data, err := ioutil.ReadFile("/Users/fabojiang/Desktop/video-files/test.h264")
	if err != nil {
		t.Errorf("Read file failed, %v", err)
	}

	wbuf := &bytes.Buffer{}
	// 先写入flv 头信息
	var flvHeader []byte
	flvHeader = append(flvHeader, []byte{'F', 'L', 'V'}...)          // 先发送flv三个字符
	flvHeader = append(flvHeader, []byte{0x01}...)                   // 版本号
	flvHeader = append(flvHeader, []byte{0x04}...)                   // Flags，第0位和第2位，分别表示video和audio的存在情况（1存在，0不存在）
	flvHeader = append(flvHeader, []byte{0x00, 0x00, 0x00, 0x09}...) // DataOffset flvHeader长度，固定位9个字节
	flvHeader = append(flvHeader, []byte{0x00, 0x00, 0x00, 0x00}...) // previous tag size0
	wbuf.Write(flvHeader)

	objmap := make(amf.Object)
	objmap["videocodecid"] = av.VideoH264
	objmap["height"] = 480
	objmap["width"] = 640
	objmap["aaa"] = "sdfdsfdsfdsf"

	bts := &bytes.Buffer{}
	encoder := amf.Encoder{}
	if _, err := encoder.Encode(bts, "onMetaData", amf.AMF0); err != nil {
		t.Errorf("encode failed, %v", err)
	}
	if _, err := encoder.Encode(bts, objmap, amf.AMF0); err != nil {
		t.Errorf("encode failed, %v", err)
	}
	mateData, _ := PackScriptData(av.TAG_SCRIPTDATAAMF0, 0, bts.Bytes())
	wbuf.Write(mateData)
	binary.Write(wbuf, binary.BigEndian, uint32(len(mateData)))

	var timeStamp uint32
	bfirst := true
	bcheck := true
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

				naluData := data[pre:index]
				pre = index
				index += 4

				naluType := naluData[4] & 0x1F

				if bfirst {
					if naluType != 7 {
						continue
					}

					// 解析出sps和pps
					var sps, pps []byte
					nalus := h264.ParseNalus(naluData)
					for _, nalu := range nalus {
						if naluType := nalu[0] & 0x1F; naluType == 7 {
							sps = nalu
						} else if naluType == 8 {
							pps = nalu
						}
					}

					if sps == nil || pps == nil {
						continue
					}

					// avc seq
					sequenceData := NewAVCSequenceHeader(sps, pps, timeStamp, true)
					wbuf.Write(sequenceData)
					binary.Write(wbuf, binary.BigEndian, uint32(len(sequenceData)))
					bfirst = false
				}

				vh := av.VideoPacketHeader{
					FrameType:       av.FrameInter, // todo 这个值需不需要传递
					AVCPacketType:   av.AvcNALU,
					CodecID:         av.VideoH264,
					CompositionTime: 0, // todo
				}
				videoData, err := PackVideoData(&vh, true, 0, naluData, timeStamp)
				if err != nil {
					t.Errorf("pack video data failed, %v", err)
				}
				wbuf.Write(videoData)
				binary.Write(wbuf, binary.BigEndian, uint32(len(videoData)))
				timeStamp += 40

				continue
			}
		}

		index++
		if index+4 >= len(data) {
			break
		}
	}

	file, err := os.OpenFile("h264.flv", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		t.Errorf("Open file failed. %v", err)
	}
	defer file.Close()

	file.Write(wbuf.Bytes())
}
