package cache

import (
	"bytes"
	"errors"
	"flag"
	"fmt"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/protocol/amf"
)

var (
	gopNum = flag.Int("gopNum", 1, "gop num")
)

// Cache ...
type Cache struct {
	gop      *GopCache
	videoSeq *av.Packet
	audioSeq *av.Packet
	metadata *av.Packet
}

// NewCache ...
func NewCache() *Cache {
	return &Cache{
		gop:      NewGopCache(*gopNum),
		videoSeq: nil,
		audioSeq: nil,
		metadata: nil,
	}
}

// SaveAudioSeq ...
func (cache *Cache) SaveAudioSeq(pkt *av.Packet) {
	cache.audioSeq = pkt
}

// SaveVideoSeq ...
func (cache *Cache) SaveVideoSeq(pkt *av.Packet) {
	cache.videoSeq = pkt
}

// SaveVideo ...
func (cache *Cache) SaveVideo(pkt *av.Packet, bKeyFrame bool) {
	cache.gop.Write(pkt, bKeyFrame)
}

// SaveMetaData ..
func (cache *Cache) SaveMetaData(pkt *av.Packet) {
	cache.metadata = &av.Packet{
		PacketType: pkt.PacketType,
		TimeStamp:  pkt.TimeStamp,
		StreamID:   pkt.StreamID,
		VHeader:    pkt.VHeader,
		AHeader:    pkt.AHeader,
		Data:       make([]byte, len(pkt.Data)),
	}
	copy(cache.metadata.Data, pkt.Data)
}

// func (cache *Cache) Write(p *av.Packet) {
// 	switch p.PacketType {
// 	case av.PacketTypeAudio:
// 		// 目前只处理aac的sequence header，如果后续要支持更多的格式
// 		// 可在此添加
// 		if p.AHeader.SoundFormat == av.SOUND_AAC && p.AHeader.AACPacketType == av.AAC_SEQHDR {
// 			cache.audioSeq = p
// 			return
// 		}
// 	case av.PacketTypeVideo:
// 		// 这里目前只处理h264的sequence和gop缓存
// 		if p.VHeader.CodecID == av.VideoH264 {
// 			if p.VHeader.FrameType == av.FrameKey {
// 				if p.VHeader.AVCPacketType == av.AvcSEQHDR {
// 					cache.videoSeq = p
// 				} else {
// 					cache.gop.Write(p, true)
// 				}
// 			}
// 			cache.gop.Write(p, false)
// 		}
// 	case av.PacketTypeMetadata:
// 		cache.metadata = p
// 	}
// }

// Send ...
func (cache *Cache) Send(inputChan chan<- *av.Packet) error {
	cachePkts := make([]*av.Packet, 3)
	cachePkts = cachePkts[:0]
	if cache.metadata != nil {
		fmt.Println("Debug.... send metadata...")
		cachePkts = append(cachePkts, cache.metadata)

		decoder := amf.NewDecoder()
		reader := bytes.NewReader(cache.metadata.Data)
		vs, err := decoder.DecodeBatch(reader, amf.AMF0)
		if err != nil {
			fmt.Println("Debug.... decode err ", err)
		}
		for _, value := range vs {
			fmt.Println("Debug...... ", value)
		}
	}
	if cache.videoSeq != nil {
		fmt.Println("Debug.... video sequence....")
		cachePkts = append(cachePkts, cache.videoSeq)
	}
	if cache.audioSeq != nil {
		fmt.Println("Debug.... audio sequence....")
		cachePkts = append(cachePkts, cache.audioSeq)
	}

	// 发送sequence header
	for _, pkt := range cachePkts {
		select {
		case inputChan <- pkt:
		default:
			return errors.New("send sequence failed")
		}
	}

	// 发送视频帧
	for _, pkt := range cache.gop.gops {
		select {
		case inputChan <- pkt:
		default:
		}
	}
	return nil
}
