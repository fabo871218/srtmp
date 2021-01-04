package cache

import (
	"errors"
	"flag"
	"fmt"

	"github.com/fabo871218/srtmp/av"
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

func (cache *Cache) Write(p *av.Packet) {
	switch p.PacketType {
	case av.PacketTypeAudio:
		// 目前只处理aac的sequence header，如果后续要支持更多的格式
		// 可在此添加
		if p.AHeader.SoundFormat == av.SOUND_AAC && p.AHeader.AACPacketType == av.AAC_SEQHDR {
			cache.audioSeq = p
			return
		}
	case av.PacketTypeVideo:
		// 这里目前只处理h264的sequence和gop缓存
		if p.VHeader.CodecID == av.VIDEO_H264 {
			if p.VHeader.FrameType == av.FRAME_KEY {
				if p.VHeader.AVCPacketType == av.AVC_SEQHDR {
					cache.videoSeq = p
				} else {
					cache.gop.Write(p, true)
				}
			}
			cache.gop.Write(p, false)
		}
	case av.PacketTypeMetadata:
		cache.metadata = p
	}
}

// Send ...
func (cache *Cache) Send(inputChan chan<- *av.Packet) error {
	cachePkts := make([]*av.Packet, 3)
	cachePkts = cachePkts[:0]
	if cache.metadata != nil {
		cachePkts = append(cachePkts, cache.metadata)
	}
	if cache.videoSeq != nil {
		cachePkts = append(cachePkts, cache.videoSeq)
	}
	if cache.audioSeq != nil {
		cachePkts = append(cachePkts, cache.audioSeq)
	}

	// 发送sequence header
	for _, pkt := range cachePkts {
		select {
		case inputChan <- pkt:
			fmt.Println("Input pkt....")
		default:
			return errors.New("send sequence failed")
		}
	}

	// 发送视频帧
	for _, pkt := range cache.gop.gops {
		select {
		case inputChan <- pkt:
			fmt.Println("Input pkt....")
		default:
		}
	}
	return nil
}
