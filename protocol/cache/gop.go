package cache

import (
	"github.com/fabo871218/srtmp/av"
)

// GopCache ...
type GopCache struct {
	maxNumber int
	count     int
	gops      []*av.Packet
}

// NewGopCache ...
func NewGopCache(maxNumber int) *GopCache {
	return &GopCache{
		count: 0,
		gops:  make([]*av.Packet, maxNumber),
	}
}

func (gc *GopCache) Write(p *av.Packet, bKeyFrame bool) {
	if bKeyFrame {
		gc.gops = gc.gops[:0]
		gc.count = 0
	}

	// todo 是否需要拷贝
	gc.gops = append(gc.gops, p)
	gc.count++
}
