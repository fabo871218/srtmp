package flv

// import (
// 	"flag"
// 	"fmt"
// 	"os"
// 	"strings"
// 	"time"

// 	"github.com/fabo871218/srtmp/av"
// 	"github.com/fabo871218/srtmp/protocol/amf"
// 	"github.com/fabo871218/srtmp/utils"
// )

// var (
// 	flvHeader = []byte{0x46, 0x4c, 0x56, 0x01, 0x05, 0x00, 0x00, 0x00, 0x09}
// 	flvFile   = flag.String("filFile", "./out.flv", "output flv file name")
// )

// func NewFlv(handler av.Handler, streamInfo av.StreamInfo) error {
// 	patths := strings.SplitN(streamInfo.Key, "/", 2)

// 	if len(patths) != 2 {
// 		return fmt.Errorf("Invalid key:%s", streamInfo.Key)
// 	}

// 	w, err := os.OpenFile(*flvFile, os.O_CREATE|os.O_RDWR, 0755)
// 	if err != nil {
// 		return fmt.Errorf("open file failed, %v", err)
// 	}
// 	//todo 文件句柄如何关闭
// 	writer := NewFLVWriter(patths[0], patths[1], streamInfo.URL, w)

// 	handler.HandleWriter(writer)

// 	writer.Wait()
// 	// close flv file
// 	writer.ctx.Close()
// 	return nil
// }

// const (
// 	headerLen = 11
// )

// type FLVWriter struct {
// 	av.RWBaser
// 	UID             string
// 	app, title, url string
// 	buf             []byte
// 	closed          chan struct{}
// 	ctx             *os.File
// }

// func NewFLVWriter(app, title, url string, ctx *os.File) *FLVWriter {
// 	ret := &FLVWriter{
// 		UID:     utils.NewId(),
// 		app:     app,
// 		title:   title,
// 		url:     url,
// 		ctx:     ctx,
// 		RWBaser: av.NewRWBaser(time.Second * 10),
// 		closed:  make(chan struct{}),
// 		buf:     make([]byte, headerLen),
// 	}

// 	ret.ctx.Write(flvHeader)
// 	utils.PutI32BE(ret.buf[:4], 0)
// 	ret.ctx.Write(ret.buf[:4])

// 	return ret
// }

// func (writer *FLVWriter) Write(p *av.Packet) error {
// 	writer.RWBaser.SetPreTime()
// 	h := writer.buf[:headerLen]
// 	typeID := av.TAG_VIDEO
// 	switch p.PacketType {
// 	case av.PacketTypeVideo:
// 		typeID = av.TAG_VIDEO
// 	case av.PacketTypeAudio:
// 		typeID = av.TAG_AUDIO
// 	case av.PacketTypeMetadata:
// 		var err error
// 		typeID = av.TAG_SCRIPTDATAAMF0
// 		p.Data, err = amf.MetaDataReform(p.Data, amf.DEL)
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	dataLen := len(p.Data)
// 	timestamp := p.TimeStamp
// 	timestamp += writer.BaseTimeStamp()
// 	writer.RWBaser.RecTimeStamp(timestamp, uint32(typeID))

// 	preDataLen := dataLen + headerLen
// 	timestampbase := timestamp & 0xffffff
// 	timestampExt := timestamp >> 24 & 0xff

// 	utils.PutU8(h[0:1], uint8(typeID))
// 	utils.PutI24BE(h[1:4], int32(dataLen))
// 	utils.PutI24BE(h[4:7], int32(timestampbase))
// 	utils.PutU8(h[7:8], uint8(timestampExt))

// 	if _, err := writer.ctx.Write(h); err != nil {
// 		return err
// 	}

// 	if _, err := writer.ctx.Write(p.Data); err != nil {
// 		return err
// 	}

// 	utils.PutI32BE(h[:4], int32(preDataLen))
// 	if _, err := writer.ctx.Write(h[:4]); err != nil {
// 		return err
// 	}

// 	return nil
// }

// func (writer *FLVWriter) Wait() {
// 	select {
// 	case <-writer.closed:
// 		return
// 	}
// }

// func (writer *FLVWriter) Close() {
// 	writer.ctx.Close()
// 	close(writer.closed)
// }

// func (writer *FLVWriter) StreamInfo() (ret av.StreamInfo) {
// 	ret.UID = writer.UID
// 	ret.URL = writer.url
// 	ret.Key = writer.app + "/" + writer.title
// 	return
// }

// type FlvDvr struct{}

// func (f *FlvDvr) NewWriter(streamInfo av.StreamInfo) (av.WriteCloser, error) {
// 	paths := strings.SplitN(streamInfo.Key, "/", 2)
// 	if len(paths) != 2 {
// 		return nil, fmt.Errorf("invalid key:%s", streamInfo.Key)
// 	}

// 	err := os.MkdirAll(paths[0], 0755)
// 	if err != nil {
// 		return nil, fmt.Errorf("mkdir failed, %v", err)
// 	}

// 	fileName := fmt.Sprintf("%s_%d.%s", streamInfo.Key, time.Now().Unix(), "flv")
// 	w, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0755)
// 	if err != nil {
// 		return nil, fmt.Errorf("open file failed, %v", err)
// 	}

// 	writer := NewFLVWriter(paths[0], paths[1], streamInfo.URL, w)
// 	return writer, nil
// }
