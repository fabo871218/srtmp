package av

import (
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

	//rtmp tag中只支持前面四种采样率，后面是为了程序处理方便自己添加
	SOUND_RATE_5_5Khz = 0 //
	SOUND_RATE_11Khz  = 1 //11025 hz
	SOUND_RATE_22Khz  = 2 //22050 hz
	SOUND_RATE_44Khz  = 3

	//自己添加
	SOUND_RATE_7Khz  = 4  //7350 hz
	SOUND_RATE_8Khz  = 5  //8000 hz
	SOUND_RATE_12Khz = 6  //12000 hz
	SOUND_RATE_16Khz = 7  //16000 hz
	SOUND_RATE_24Khz = 8  //24000 hz
	SOUND_RATE_32Khz = 9  //32000 hz
	SOUND_RATE_48Khz = 10 //48000 hz
	SOUND_RATE_64Khz = 11 //64000 hz
	SOUND_RATE_88Khz = 12 //88200 hz
	SOUND_RATE_96Khz = 13 // 96000 hz

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
	VIDEO_JPEG        = 1
	VideoH263         = 2
	VideoScreen       = 3
	VideoVP6          = 4
	VideoVP6WithAlpha = 5
	VideoScreenV2     = 6
	VIDEO_H264        = 7
)

// Packet类型
const (
	PacketTypeUnknow   = 0
	PacketTypeVideo    = 1 //音频包
	PacketTypeAudio    = 2 //视频包
	PacketTypeMetadata = 3 //数据包
)

var (
	PUBLISH = "publish"
	PLAY    = "play"
)

// Header can be converted to AudioHeaderInfo or VideoHeaderInfo
type Packet struct {
	PacketType uint32 // packet类型
	TimeStamp  uint32 // dts
	StreamID   uint32
	VHeader    VideoPacketHeader
	AHeader    AudioPacketHeader
	Data       []byte
}

//AudioPacketHeader comment
type AudioPacketHeader struct {
	SoundFormat   uint8
	SoundRate     uint8
	SoundSize     uint8
	SoundType     uint8
	AACPacketType uint8
}

//VideoPacketHeader ...
type VideoPacketHeader struct {
	FrameType       uint8
	AVCPacketType   uint8
	CodecID         uint8
	CompositionTime int32
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
