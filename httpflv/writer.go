// package httpflv

// import (
// 	"errors"
// 	"fmt"
// 	"net/http"
// 	"time"

// 	"github.com/fabo871218/srtmp/av"
// 	"github.com/fabo871218/srtmp/protocol/amf"
// 	"github.com/fabo871218/srtmp/utils"
// )

// const (
// 	headerLen   = 11
// 	maxQueueNum = 1024
// )

// type FLVWriter struct {
// 	Uid string
// 	av.RWBaser
// 	app, title, url string
// 	buf             []byte
// 	closed          bool
// 	closedChan      chan struct{}
// 	ctx             http.ResponseWriter
// 	packetQueue     chan *av.Packet
// }

// func NewFLVWriter(app, title, url string, ctx http.ResponseWriter) *FLVWriter {
// 	ret := &FLVWriter{
// 		Uid:         utils.NewId(),
// 		app:         app,
// 		title:       title,
// 		url:         url,
// 		ctx:         ctx,
// 		RWBaser:     av.NewRWBaser(time.Second * 10),
// 		closedChan:  make(chan struct{}),
// 		buf:         make([]byte, headerLen),
// 		packetQueue: make(chan *av.Packet, maxQueueNum),
// 	}

// 	ret.ctx.Write([]byte{0x46, 0x4c, 0x56, 0x01, 0x05, 0x00, 0x00, 0x00, 0x09})
// 	utils.PutI32BE(ret.buf[:4], 0)
// 	ret.ctx.Write(ret.buf[:4])
// 	go func() {
// 		err := ret.SendPacket()
// 		if err != nil {
// 			fmt.Printf("SendPacket failed, %v\n", err)
// 			ret.closed = true
// 		}
// 	}()
// 	return ret
// }

// func (flvWriter *FLVWriter) DropPacket(pktQue chan *av.Packet, streamInfo av.StreamInfo) {
// 	fmt.Printf("[%v] packet queue max!!!\n", streamInfo)
// 	for i := 0; i < maxQueueNum-84; i++ {
// 		tmpPkt, ok := <-pktQue
// 		if !ok {
// 			continue
// 		}

// 		switch tmpPkt.PacketType {
// 		case av.PacketTypeVideo:
// 			videoPkt, ok := tmpPkt.Header.(av.VideoPacketHeader)
// 			// dont't drop sps config and dont't drop key frame
// 			if ok && videoPkt.FrameType == av.FRAME_KEY {
// 				fmt.Println("insert keyframe to queue")
// 				pktQue <- tmpPkt
// 			}

// 			if len(pktQue) > maxQueueNum-10 {
// 				<-pktQue
// 			}
// 			// drop other packet
// 			<-pktQue
// 		case av.PacketTypeAudio:
// 			fmt.Println("insert audio to queue")
// 			pktQue <- tmpPkt
// 		default:
// 		}
// 	}
// 	fmt.Printf("packet queue len: %d\n", len(pktQue))
// }

// func (flvWriter *FLVWriter) Write(p *av.Packet) (err error) {
// 	err = nil
// 	if flvWriter.closed {
// 		err = errors.New("flvwrite source closed")
// 		return
// 	}
// 	defer func() {
// 		if e := recover(); e != nil {
// 			errString := fmt.Sprintf("FLVWriter has already been closed:%v", e)
// 			err = errors.New(errString)
// 		}
// 	}()
// 	if len(flvWriter.packetQueue) >= maxQueueNum-24 {
// 		flvWriter.DropPacket(flvWriter.packetQueue, flvWriter.StreamInfo())
// 	} else {
// 		flvWriter.packetQueue <- p
// 	}

// 	return
// }

// func (flvWriter *FLVWriter) SendPacket() error {
// 	for {
// 		p, ok := <-flvWriter.packetQueue
// 		if ok {
// 			flvWriter.RWBaser.SetPreTime()
// 			h := flvWriter.buf[:headerLen]
// 			typeID := av.TAG_VIDEO
// 			switch p.PacketType {
// 			case av.PacketTypeVideo:
// 				typeID = av.TAG_VIDEO
// 			case av.PacketTypeAudio:
// 				typeID = av.TAG_AUDIO
// 			case av.PacketTypeMetadata:
// 				var err error
// 				typeID = av.TAG_SCRIPTDATAAMF0
// 				p.Data, err = amf.MetaDataReform(p.Data, amf.DEL)
// 				if err != nil {
// 					return err
// 				}
// 			}
// 			dataLen := len(p.Data)
// 			timestamp := p.TimeStamp
// 			timestamp += flvWriter.BaseTimeStamp()
// 			flvWriter.RWBaser.RecTimeStamp(timestamp, uint32(typeID))

// 			preDataLen := dataLen + headerLen
// 			timestampbase := timestamp & 0xffffff
// 			timestampExt := timestamp >> 24 & 0xff

// 			utils.PutU8(h[0:1], uint8(typeID))
// 			utils.PutI24BE(h[1:4], int32(dataLen))
// 			utils.PutI24BE(h[4:7], int32(timestampbase))
// 			utils.PutU8(h[7:8], uint8(timestampExt))

// 			if _, err := flvWriter.ctx.Write(h); err != nil {
// 				return err
// 			}

// 			if _, err := flvWriter.ctx.Write(p.Data); err != nil {
// 				return err
// 			}

// 			utils.PutI32BE(h[:4], int32(preDataLen))
// 			if _, err := flvWriter.ctx.Write(h[:4]); err != nil {
// 				return err
// 			}
// 		} else {
// 			return errors.New("closed")
// 		}
// 	}
// }

// func (flvWriter *FLVWriter) Wait() {
// 	select {
// 	case <-flvWriter.closedChan:
// 		return
// 	}
// }

// func (flvWriter *FLVWriter) Close() {
// 	fmt.Println("http flv closed")
// 	if !flvWriter.closed {
// 		close(flvWriter.packetQueue)
// 		close(flvWriter.closedChan)
// 	}
// 	flvWriter.closed = true
// }

// func (flvWriter *FLVWriter) StreamInfo() (ret av.StreamInfo) {
// 	ret.UID = flvWriter.Uid
// 	ret.URL = flvWriter.url
// 	ret.Key = flvWriter.app + "/" + flvWriter.title
// 	ret.Inter = true
// 	return
// }
