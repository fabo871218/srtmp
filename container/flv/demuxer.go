package flv

import (
	"errors"
	"fmt"

	"github.com/fabo871218/srtmp/av"
)

//ErrAvcEndSEQ ...
var ErrAvcEndSEQ = errors.New("avc end sequence")

type Demuxer struct {
}

//NewDemuxer ...
func NewDemuxer() *Demuxer {
	return &Demuxer{}
}

//DemuxH ...
func (d *Demuxer) DemuxH(p *av.Packet) (err error) {
	var tag Tag
	if p.IsAudio {
		_, err = tag.ParseAudioHeader(p.Data)
	} else if p.IsVideo {
		_, err = tag.ParseVideoHeader(p.Data)
	} else {
		//todo IsMetadata如何处理
		return fmt.Errorf("Unsupport type")
	}
	p.Header = &tag
	return
}

//Demux ...
func (d *Demuxer) Demux(p *av.Packet) (err error) {
	var (
		tag Tag
		n   int
	)
	if p.IsAudio {
		n, err = tag.ParseAudioHeader(p.Data)
	} else if p.IsVideo {
		n, err = tag.ParseVideoHeader(p.Data)
	} else {
		return fmt.Errorf("Unsupport type")
	}
	if err != nil {
		return
	}
	p.Header = &tag
	p.Data = p.Data[n:]
	return
}
