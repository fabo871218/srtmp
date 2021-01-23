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
		if _, err = tag.ParseAudioHeader(p.Data); err != nil {
			return
		}
		p.AHeader = av.AudioPacketHeader{
			SoundFormat:   tag.mediat.soundFormat,
			SoundRate:     tag.mediat.soundRate,
			SoundSize:     tag.mediat.soundSize,
			SoundType:     tag.mediat.soundType,
			AACPacketType: tag.mediat.aacPacketType,
		}
	case av.PacketTypeVideo:
		if _, err = tag.ParseVideoHeader(p.Data); err != nil {
			return
		}
		p.VHeader = av.VideoPacketHeader{
			FrameType:       tag.mediat.frameType,
			AVCPacketType:   tag.mediat.avcPacketType,
			CodecID:         tag.mediat.codecID,
			CompositionTime: tag.mediat.compositionTime,
		}
	default:
		//todo IsMetadata如何处理
		return fmt.Errorf("Unsupport type")
	}
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
		if n, err = tag.ParseAudioHeader(p.Data); err != nil {
			return
		}
		p.AHeader = av.AudioPacketHeader{
			SoundFormat:   tag.mediat.soundFormat,
			SoundRate:     tag.mediat.soundRate,
			SoundSize:     tag.mediat.soundSize,
			SoundType:     tag.mediat.soundType,
			AACPacketType: tag.mediat.aacPacketType,
		}
	case av.PacketTypeVideo:
		if n, err = tag.ParseVideoHeader(p.Data); err != nil {
			return
		}
		p.VHeader = av.VideoPacketHeader{
			FrameType:       tag.mediat.frameType,
			AVCPacketType:   tag.mediat.avcPacketType,
			CodecID:         tag.mediat.codecID,
			CompositionTime: tag.mediat.compositionTime,
		}
	default:
		return fmt.Errorf("Unsupport type:%d", p.PacketType)
	}
	if err != nil {
		return
	}
	p.Data = p.Data[n:]
	return
}
