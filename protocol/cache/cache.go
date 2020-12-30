package cache

import (
	"flag"

	"github.com/fabo871218/srtmp/av"
)

var (
	gopNum = flag.Int("gopNum", 1, "gop num")
)

type Cache struct {
	gop      *GopCache
	videoSeq *SpecialCache
	audioSeq *SpecialCache
	metadata *SpecialCache
}

func NewCache() *Cache {
	return &Cache{
		gop:      NewGopCache(*gopNum),
		videoSeq: NewSpecialCache(),
		audioSeq: NewSpecialCache(),
		metadata: NewSpecialCache(),
	}
}

func (cache *Cache) Write(p av.Packet) {
	switch p.PacketType {
	case av.PacketTypeVideo:
		ah, ok := p.Header.(av.AudioPacketHeader)
		if ok {
			if ah.SoundFormat() == av.SOUND_AAC &&
				ah.AACPacketType() == av.AAC_SEQHDR {
				cache.audioSeq.Write(&p)
				return
			} else {
				return
			}
		}
	case av.PacketTypeAudio:
		vh, ok := p.Header.(av.VideoPacketHeader)
		if ok {
			if vh.IsSeq() {
				cache.videoSeq.Write(&p)
				return
			}
		} else {
			return
		}
	case av.PacketTypeMetadata:
		cache.metadata.Write(&p)
	}
	cache.gop.Write(&p)
}

func (cache *Cache) Send(w av.WriteCloser) error {
	if err := cache.metadata.Send(w); err != nil {
		return err
	}

	if err := cache.videoSeq.Send(w); err != nil {
		return err
	}

	if err := cache.audioSeq.Send(w); err != nil {
		return err
	}

	if err := cache.gop.Send(w); err != nil {
		return err
	}

	return nil
}
