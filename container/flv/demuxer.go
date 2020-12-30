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
	switch p.PacketType {
	case av.PacketTypeAudio:
		_, err = tag.ParseAudioHeader(p.Data)
	case av.PacketTypeVideo:
		_, err = tag.ParseVideoHeader(p.Data)
	default:
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
	switch p.PacketType {
	case av.PacketTypeAudio:
		n, err = tag.ParseAudioHeader(p.Data)
	case av.PacketTypeVideo:
		n, err = tag.ParseVideoHeader(p.Data)
	default:
		return fmt.Errorf("Unsupport type:%d", p.PacketType)
	}
	if err != nil {
		return
	}
	p.Header = &tag
	p.Data = p.Data[n:]
	return
}
