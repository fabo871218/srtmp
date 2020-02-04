package av

import (
	"fmt"
	"io"
)

const (
	TAG_AUDIO          = 8
	TAG_VIDEO          = 9
	TAG_SCRIPTDATAAMF0 = 18
	TAG_SCRIPTDATAAMF3 = 0xf
)

const (
	MetadatAMF0  = 0x12
	MetadataAMF3 = 0xf
)

const (
	SOUND_MP3                   = 2
	SOUND_NELLYMOSER_16KHZ_MONO = 4
	SOUND_NELLYMOSER_8KHZ_MONO  = 5
	SOUND_NELLYMOSER            = 6
	SOUND_ALAW                  = 7
	SOUND_MULAW                 = 8
	SOUND_AAC                   = 10
	SOUND_SPEEX                 = 11

	SOUND_5_5Khz = 0
	SOUND_11Khz  = 1
	SOUND_22Khz  = 2
	SOUND_44Khz  = 3

	SOUND_8BIT  = 0
	SOUND_16BIT = 1

	SOUND_MONO   = 0
	SOUND_STEREO = 1

	AAC_SEQHDR = 0
	AAC_RAW    = 1
)

const (
	//视频tag的帧类型, 对于avc(h264)只用到了前面两个
	FRAME_KEY   = 1 // keyframe （for avc, a seekable frame)
	FRAME_INTER = 2 // inter frame (for avc, a non-seekable frame)
	//3:disposable inter frame
	//4:generated keyframe(reserved for server use only)
	//5:vidoe info/command frame

	//avc 视频封装格式
	AVC_SEQHDR = 0 // avc sequence header
	AVC_NALU   = 1 // avc nalu
	AVC_EOS    = 2 // avc end of sequence

	//avc视频编码id
	VIDEO_H264 = 7
)

var (
	PUBLISH = "publish"
	PLAY    = "play"
)

// Header can be converted to AudioHeaderInfo or VideoHeaderInfo
type Packet struct {
	IsAudio    bool
	IsVideo    bool
	IsMetadata bool
	TimeStamp  uint32 // dts
	StreamID   uint32
	Header     PacketHeader
	Data       []byte
}

type StreamInfo struct {
	Key   string
	URL   string
	UID   string
	Inter bool
}

type PacketHeader interface {
}

type AudioPacketHeader interface {
	PacketHeader
	SoundFormat() uint8
	AACPacketType() uint8
}

type VideoPacketHeader interface {
	PacketHeader
	IsKeyFrame() bool
	IsSeq() bool
	CodecID() uint8
	CompositionTime() int32
}

type Demuxer interface {
	Demux(*Packet) (ret *Packet, err error)
}

type Muxer interface {
	Mux(*Packet, io.Writer) error
}

type SampleRater interface {
	SampleRate() (int, error)
}

type CodecParser interface {
	SampleRater
	Parse(*Packet, io.Writer) error
}

type ExtendWriter interface {
	NewWriter(StreamInfo) (WriteCloser, error)
}

type Handler interface {
	HandleReader(ReadCloser) error
	HandleWriter(WriteCloser) error
}

func (streamInfo StreamInfo) IsInterval() bool {
	return streamInfo.Inter
}

func (streamInfo StreamInfo) String() string {
	return fmt.Sprintf("<key: %s, URL: %s, UID: %s, Inter: %v>",
		streamInfo.Key, streamInfo.URL, streamInfo.UID, streamInfo.Inter)
}

type ReadCloser interface {
	StreamInfo() StreamInfo
	Close()
	Alive() bool
	Read(*Packet) error
}

type WriteCloser interface {
	StreamInfo() StreamInfo
	Close()
	Alive() bool
	CalcBaseTimestamp()
	Write(*Packet) error
}
