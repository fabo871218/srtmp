package cache

// import (
// 	"bytes"
// 	"log"

// 	"github.com/fabo871218/srtmp/av"
// 	"github.com/fabo871218/srtmp/protocol"
// 	"github.com/fabo871218/srtmp/protocol/amf"
// )

// const (
// 	SetDataFrame string = "@setDataFrame"
// 	OnMetaData   string = "onMetaData"
// )

// var setFrameFrame []byte

// func init() {
// 	b := bytes.NewBuffer(nil)
// 	encoder := &amf.Encoder{}
// 	if _, err := encoder.Encode(b, SetDataFrame, amf.AMF0); err != nil {
// 		log.Fatal(err)
// 	}
// 	setFrameFrame = b.Bytes()
// }

// // SpecialCache ...
// type SpecialCache struct {
// 	full bool
// 	p    *av.Packet
// }

// // NewSpecialCache ...
// func NewSpecialCache() *SpecialCache {
// 	return &SpecialCache{}
// }

// func (specialCache *SpecialCache) Write(p *av.Packet) {
// 	specialCache.p = p
// 	specialCache.full = true
// }

// // Send ...
// func (specialCache *SpecialCache) Send(w protocol.WriteCloser) error {
// 	if !specialCache.full {
// 		return nil
// 	}
// 	return w.Write(specialCache.p)
// }
