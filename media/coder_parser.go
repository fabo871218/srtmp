package parser

import (
	"errors"
	"fmt"
	"io"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/media/aac"
	"github.com/fabo871218/srtmp/media/h264"
	"github.com/fabo871218/srtmp/media/mp3"
)

var (
	errNoAudio = errors.New("demuxer no audio")
)

type CodecParser struct {
	aac  *aac.Parser
	mp3  *mp3.Parser
	h264 *h264.Parser
}

func NewCodecParser() *CodecParser {
	return &CodecParser{}
}

func (codeParser *CodecParser) SampleRate() (int, error) {
	if codeParser.aac == nil && codeParser.mp3 == nil {
		return 0, errNoAudio
	}
	if codeParser.aac != nil {
		return codeParser.aac.SampleRate(), nil
	}
	return codeParser.mp3.SampleRate(), nil
}

func (codeParser *CodecParser) Parse(p *av.Packet, w io.Writer) (err error) {
	switch p.PacketType {
	case av.PacketTypeVideo:

		if p.VHeader.CodecID == av.VIDEO_H264 {
			if codeParser.h264 == nil {
				codeParser.h264 = h264.NewParser()
			}
			isSeq := p.VHeader.FrameType == av.FRAME_KEY && p.VHeader.AVCPacketType == av.AVC_SEQHDR
			err = codeParser.h264.Parse(p.Data, isSeq, w)
		}

	case av.PacketTypeAudio:
		switch p.AHeader.SoundFormat {
		case av.SOUND_AAC:
			if codeParser.aac == nil {
				codeParser.aac = aac.NewParser()
			}
			err = codeParser.aac.Parse(p.Data, p.AHeader.AACPacketType, w)
		case av.SOUND_MP3:
			if codeParser.mp3 == nil {
				codeParser.mp3 = mp3.NewParser()
			}
			err = codeParser.mp3.Parse(p.Data)
		}
	default:
		err = fmt.Errorf("Unknow packet type:%d", p.PacketType)
	}
	return
}
