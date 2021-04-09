package httpflv

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/media/h264"
	"github.com/fabo871218/srtmp/protocol/amf"
)

var startCode []byte = []byte{0x00, 0x00, 0x00, 0x01}

func handleConn(conn net.Conn) {
	httpReader := &HttpReader{}

	for {
		if err := httpReader.ReadMessage(conn); err != nil {
			fmt.Println("Debug.... err, ", err)
			return
		}

		if err := writeFlv(conn); err != nil {
			fmt.Println("write flv failed, ", err)
			return
		}
	}
}

func writeFlv(w io.Writer) error {
	httpHeader := "HTTP/1.1 200 OK\r\n" +
		"Access-Control-Allow-Origin: *\r\n" +
		"Cache-Control: no-cache\r\n" +
		"Content-Type: video/x-flv\r\n" +
		"Date: Wed, 17 Mar 2021 11:46:52 GMT\r\n" +
		"Connection: close\r\n" +
		"Transfer-Encoding: chunked\r\n" +
		"\r\n"

	if _, err := w.Write([]byte(httpHeader)); err != nil {
		return err
	}
	cw := NewChunkWriter(w)

	flvFilePlay(cw)

	w.Write([]byte("0\r\n\r\n"))
	return nil
}

func ListenAndServe1(addr string) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		}

		go handleConn(conn)
	}
}

func ListenAndServe(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/flv/", Playflv)
	mux.HandleFunc("/file/", FlvFile)

	server := http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  time.Second * 60,
		WriteTimeout: time.Second * 60,
	}

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

func FlvFile(w http.ResponseWriter, r *http.Request) {
	bts := &bytes.Buffer{}
	flvPlay(bts)

	wfile, err := os.OpenFile("http.flv", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}
	defer wfile.Close()

	wfile.Write(bts.Bytes())
}

func Playflv(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "video/x-flv")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flvPlay(w)

	//flvFilePlay(w)
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

					fmt.Println("write sequence.....")
					sequenceData := flv.NewAVCSequenceHeader(sps, pps, timeStamp, true)
					//crln := fmt.Sprintf("%d\r\n", len(sequenceData))
					//w.Write([]byte(crln))
					if _, err := w.Write(sequenceData); err != nil {
						panic(err)
					}
					//w.Write([]byte("\r\n"))
					fmt.Println("Debug.... seqlen:", len(sequenceData))
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

func flvFilePlay(w io.Writer) {
	//data, err := ioutil.ReadFile("/Users/fabojiang/Desktop/xigua.flv")
	data, err := ioutil.ReadFile("http.flv")
	if err != nil {
		panic(err)
	}

	index := 0
	length := len(data)
	for index < length {
		sendlen := length - index
		if sendlen >= 2048 {
			sendlen = 2048
		}

		w.Write(data[index : index+sendlen])
		index += sendlen
	}
}

func flvPlay(w io.Writer) {
	fmt.Println("Receive new flv request....")

	// w.Header().Set("Content-Type", "video/x-flv")
	// w.Header().Set("Cache-Control", "no-cache")
	// w.Header().Set("Connection", "keep-alive")
	//w.Header().Set("Access-Control-Allow-Origin", "*")

	//w := &bytes.Buffer{}

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
