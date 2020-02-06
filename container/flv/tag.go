package flv

import (
	"fmt"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/media/aac"
	"github.com/fabo871218/srtmp/media/h264"
	"github.com/fabo871218/srtmp/utils"
)

type flvTag struct {
	fType           uint8  //8bit tag类型，包括音频tag（8），视频tag（9），脚本tag（18）
	dataSize        uint32 //24bit 数据长度，从streamID后面算起
	timeStamp       uint32 //24bit 时间戳，单位是毫秒，对于脚本类型tag，总是为0
	timeStampExtend uint8  //8bit 时间戳扩展，将时间戳扩展为4bytes，代表时间戳高8位
	streamID        uint32 //24bit always 0
}

/*
script tag 存放flv的MetaData信息，比如duration，audiodatarate，creator，width等信息

video tag
第一个byte是视频信息, 格式如下:
帧类型 (4bits) 取值:
	1: keyframe (for AVC, a seekable frame)
	2: inter frame (for AVC, a non-seekable frame)
	3: disposable inter frame (H.263 only)
	4: generated keyframe (reserved for server use only)
	5: video info/command frame
编码ID (4 bits) 取值:
	1: JPEG (currently unused)
	2: Sorenson H.263
	3: Screen video
	4: On2 VP6
	5: On2 VP6 with alpha channel
	6: Screen video version 2
	7: AVC
接下来就是具体的video的流数据的封装
对于AVC(h264)格式的video，除了第一个字节的帧类型和编码id以外，从第二个字节开始，分别为

AVC包类型:AVCPacketType (8Bits) 取值:
	0: AVC sequence header （这个必须在发送第一帧前发送，包含sps，pps解码相关信息）
	1: AVC NALU
	2: AVC end of sequence
CompositionTime (24Bits) 取值:
	如果上面的AVCPacketType=0x01, 为相对时间戳;
	其它: 均为0;
Data (n Bytes) 为负载数据 取值:
	如果AVCPacketType=0x00, 为AVCDecorderConfigurationRecord;
	如果AVCPacketType=0x01, 为NALUs;
	如果AVCPacketType=0x02, 为空.

AVCDecoderConfigurationRecord详细说明:
一般第一个视频Tag会封装视频编码的总体描述信息(AVC sequence header), 就是AVCDecoderConfigurationRecord结构(ISO/IEC 14496-15 AVC file format中规定). 其结构如下:

aligned(8) class AVCDecoderConfigurationRecord {
    unsigned int(8) configurationVersion = 1;
    unsigned int(8) AVCProfileIndication;
    unsigned int(8) profile_compatibility;
    unsigned int(8) AVCLevelIndication;

    bit(6) reserved = ‘111111’b;
    unsigned int(2) lengthSizeMinusOne;

    bit(3) reserved = ‘111’b;
    unsigned int(5) numOfSequenceParameterSets;

    for (i=0; i< numOfSequenceParameterSets; i++) {
        unsigned int(16) sequenceParameterSetLength ;
        bit(8*sequenceParameterSetLength) sequenceParameterSetNALUnit;
    }

    unsigned int(8) numOfPictureParameterSets;

    for (i=0; i< numOfPictureParameterSets; i++) {
        unsigned int(16) pictureParameterSetLength;
        bit(8*pictureParameterSetLength) pictureParameterSetNALUnit;
    }
}
*/

/*
audio tag
SoundFormat  4bit
SoundRage    2bit
SoundSize    1bit
SoundType    1bit
SoundData    n bytes //音频数据

当SoundFormat == 10 时，SoundData的数据时AAC格式
AACAudioData格式如下
AACPacketType  8bit  0--aac sequence header  1--aac raw
Data           n bytes
当 AACPacketType == 0 时，Data数据为AudioSpecificConfig
当 AAXCPacketType == 1 时，Data数据为AAC raw frame data

AudioSpecificConfig 格式为
audioObjectType  5bit
samplingFrequencyIndex 4bit
if samplingFrequencyIndex == 15 {
	当samplingFrequencyIndex， samplingFrequency直接指定采用率，否则不占位
	samplingFrequency  24bit
}
channelConfiguration 4bit //输出声道信息，如双声道为2
（GASpecificConfig 包含以下三项）
frameLengthFlag 1bit
dependsOnCoreCoder 1bit
extensionFlag 1bit
*/

//meidaTag 包含视频tag和音频tag
type mediaTag struct {
	/*
		SoundFormat: UB[4]
		0 = Linear PCM, platform endian
		1 = ADPCM
		2 = MP3
		3 = Linear PCM, little endian
		4 = Nellymoser 16-kHz mono
		5 = Nellymoser 8-kHz mono
		6 = Nellymoser
		7 = G.711 A-law logarithmic PCM
		8 = G.711 mu-law logarithmic PCM
		9 = reserved
		10 = AAC
		11 = Speex
		14 = MP3 8-Khz
		15 = Device-specific sound
		Formats 7, 8, 14, and 15 are reserved for internal use
		AAC is supported in Flash Player 9,0,115,0 and higher.
		Speex is supported in Flash Player 10 and higher.
	*/
	soundFormat uint8

	/*
		SoundRate: UB[2]
		Sampling rate
		0 = 5.5-kHz For AAC: always 3
		1 = 11-kHz
		2 = 22-kHz
		3 = 44-kHz
	*/
	soundRate uint8

	/*
		SoundSize: UB[1]
		0 = snd8Bit
		1 = snd16Bit
		Size of each sample.
		This parameter only pertains to uncompressed formats.
		Compressed formats always decode to 16 bits internally
	*/
	soundSize uint8

	/*
		SoundType: UB[1]
		0 = sndMono
		1 = sndStereo
		Mono or stereo sound For Nellymoser: always 0
		For AAC: always 1
	*/
	soundType uint8

	/*
		0: AAC sequence header
		1: AAC raw
	*/
	aacPacketType uint8

	/*
		1: keyframe (for AVC, a seekable frame)
		2: inter frame (for AVC, a non- seekable frame)
		3: disposable inter frame (H.263 only)
		4: generated keyframe (reserved for server use only)
		5: video info/command frame
	*/
	frameType uint8

	/*
		1: JPEG (currently unused)
		2: Sorenson H.263
		3: Screen video
		4: On2 VP6
		5: On2 VP6 with alpha channel
		6: Screen video version 2
		7: AVC
	*/
	codecID uint8

	/*
		0: AVC sequence header
		1: AVC NALU
		2: AVC end of sequence (lower level NALU sequence ender is not required or supported)
	*/
	avcPacketType uint8

	compositionTime int32
}

type Tag struct {
	flvt   flvTag
	mediat mediaTag
}

func (tag *Tag) SoundFormat() uint8 {
	return tag.mediat.soundFormat
}

func (tag *Tag) AACPacketType() uint8 {
	return tag.mediat.aacPacketType
}

func (tag *Tag) IsKeyFrame() bool {
	return tag.mediat.frameType == av.FRAME_KEY
}

func (tag *Tag) IsSeq() bool {
	return tag.mediat.frameType == av.FRAME_KEY &&
		tag.mediat.avcPacketType == av.AVC_SEQHDR
}

func (tag *Tag) CodecID() uint8 {
	return tag.mediat.codecID
}

func (tag *Tag) CompositionTime() int32 {
	return tag.mediat.compositionTime
}

// ParseMeidaTagHeader, parse video, audio, tag header
func (tag *Tag) ParseMeidaTagHeader(b []byte, isVideo bool) (n int, err error) {
	switch isVideo {
	case false:
		n, err = tag.parseAudioHeader(b)
	case true:
		n, err = tag.parseVideoHeader(b)
	}
	return
}

func (tag *Tag) parseAudioHeader(b []byte) (n int, err error) {
	if len(b) < n+1 {
		err = fmt.Errorf("invalid audiodata len=%d", len(b))
		return
	}
	flags := b[0]
	tag.mediat.soundFormat = flags >> 4
	tag.mediat.soundRate = (flags >> 2) & 0x3
	tag.mediat.soundSize = (flags >> 1) & 0x1
	tag.mediat.soundType = flags & 0x1
	n++
	switch tag.mediat.soundFormat {
	case av.SOUND_AAC:
		tag.mediat.aacPacketType = b[1]
		n++
	}
	return
}

func (tag *Tag) parseVideoHeader(b []byte) (n int, err error) {
	if len(b) < n+5 {
		err = fmt.Errorf("invalid videodata len=%d", len(b))
		return
	}
	//第一个字节包含帧类型（4bit）和编码id（4bit）
	flags := b[0]
	tag.mediat.frameType = flags >> 4 //获取帧类型
	tag.mediat.codecID = flags & 0xf  //获取编码id
	n++
	if tag.mediat.codecID == av.VIDEO_H264 {
		//如果编码id是avc，再获取avc的视频封装格式
		tag.mediat.avcPacketType = b[1] //AVCPacketType 0-sequence header 1-nalue 2-end of sequence
		//获取3个字节的compositionTime
		for i := 2; i < 5; i++ {
			tag.mediat.compositionTime = tag.mediat.compositionTime<<8 + int32(b[i])
		}
		n += 4
	}
	// if tag.mediat.frameType == av.FRAME_INTER || tag.mediat.frameType == av.FRAME_KEY {
	// 	tag.mediat.avcPacketType = b[1]
	// 	for i := 2; i < 5; i++ {
	// 		tag.mediat.compositionTime = tag.mediat.compositionTime<<8 + int32(b[i])
	// 	}
	// 	n += 4
	// }
	return
}

//NewAVCSequenceHeader comment
func NewAVCSequenceHeader(sps, pps []byte, timeStamp uint32) []byte {
	avcConfigRecord := h264.AVCDecoderConfigurationRecord(sps, pps)
	tag := &Tag{
		flvt: flvTag{
			fType:           av.TAG_VIDEO,                 //uint8  //8bit tag类型，包括音频tag（8），视频tag（9），脚本tag（18）
			dataSize:        uint32(len(avcConfigRecord)), //uint32 //24bit 数据长度，从streamID后面算起
			timeStamp:       timeStamp,                    //uint32 //24bit 时间戳，单位是毫秒，对于脚本类型tag，总是为0
			timeStampExtend: 0,                            //8bit 时间戳扩展，将时间戳扩展为4bytes，代表时间戳高8位
			streamID:        0,                            //24bit always 0
		},
		mediat: mediaTag{
			frameType:       av.FRAME_KEY,
			codecID:         av.VIDEO_H264,
			avcPacketType:   av.AVC_SEQHDR,
			compositionTime: 0,
		},
	}
	index := 0
	tagBuffer := muxerTagData(tag)
	buffer := make([]byte, len(tagBuffer)+4+len(avcConfigRecord))
	copy(buffer, tagBuffer)
	index += len(tagBuffer)
	//utils.PutU32BE(buffer[index:], uint32(len(avcConfigRecord)))
	//index += 4
	copy(buffer[index:], avcConfigRecord)
	index += len(avcConfigRecord)
	return buffer[:index]
}

//NewAACSequenceHeader comment
func NewAACSequenceHeader(timeStamp uint32) []byte {
	specificConfig := aac.SpecificConfig()
	tag := &Tag{
		flvt: flvTag{
			fType:           av.TAG_AUDIO,
			dataSize:        uint32(len(specificConfig)),
			timeStamp:       timeStamp,
			timeStampExtend: 0,
			streamID:        0,
		},
		mediat: mediaTag{
			soundFormat: av.SOUND_AAC,   //aac
			soundRate:   av.SOUND_44Khz, //44KHz
			soundSize:   av.SOUND_16BIT,
			soundType:   av.SOUND_MONO, //单声道

			aacPacketType: av.AAC_SEQHDR,
		},
	}

	index := 0
	tagBuffer := muxerTagData(tag)
	buffer := make([]byte, len(tagBuffer)+len(specificConfig))
	copy(buffer, tagBuffer)
	index += len(tagBuffer)
	copy(buffer[index:], specificConfig)
	index += len(specificConfig)
	return buffer[:index]
}

//NewAVCNaluData 把nalu单元打包成rtmp的payload
func NewAVCNaluData(src []byte, timeStamp uint32) (buffer []byte) {
	//nalu单元至少要大于4个字节，包括start code（一帧开始的起始码应该是4位）
	if len(src) <= 4 {
		buffer = make([]byte, 0)
		return
	}
	//获取naluType类型
	frameType := uint8(av.FRAME_INTER)
	naluType := src[4] & 0x1F
	if naluType == 7 || naluType == 8 || naluType == 5 {
		frameType = uint8(av.FRAME_KEY)
	}
	tag := &Tag{
		flvt: flvTag{
			fType:           av.TAG_VIDEO,
			dataSize:        uint32(len(src)), //在用rtmp协议发送是，改字段好像不起作用，正常情况是后面mediaTag+数据的长度
			timeStamp:       timeStamp,
			timeStampExtend: 0,
			streamID:        0,
		},
		mediat: mediaTag{
			frameType:       frameType,
			codecID:         av.VIDEO_H264,
			avcPacketType:   av.AVC_NALU,
			compositionTime: 0,
		},
	}
	//生成tagHeader 部分
	tagBuffer := muxerTagData(tag)
	index := 0
	if naluType == 7 || naluType == 8 {
		//如果是7或8，应该是I帧，把里面的nalu单元都提取出来，打包
		//naluLength nalu naluLength nalu
		nalus := h264.ParseNalusN(src, 3)
		buffer = make([]byte, len(tagBuffer)+len(src)+len(nalus)*4)
		copy(buffer[index:], tagBuffer)
		index += len(tagBuffer)
		for _, nalu := range nalus {
			if len := len(nalu); len > 0 {
				utils.PutU32BE(buffer[index:], uint32(len))
				index += 4
				copy(buffer[index:], nalu)
				index += len
			}
		}
		return buffer[:index]
	}
	src = src[4:] //去掉start code
	//创建buffer len(tagBuffer) + 4字节长度 + 数据长度
	buffer = make([]byte, len(tagBuffer)+4+len(src))
	//拷贝tag 头数据
	copy(buffer[index:], tagBuffer)
	index += len(tagBuffer)
	//设置数据长度
	utils.PutU32BE(buffer[index:], uint32(len(src)))
	index += 4
	//拷贝数据
	copy(buffer[index:], src)
	index += len(src)
	return buffer[:index]
}

//NewAACData comment
func NewAACData(src []byte, timeStamp uint32) (buffer []byte) {
	tag := &Tag{
		flvt: flvTag{
			fType:           av.TAG_AUDIO,
			dataSize:        uint32(len(src)),
			timeStamp:       timeStamp,
			timeStampExtend: 0,
			streamID:        0,
		},
		mediat: mediaTag{
			soundFormat: av.SOUND_AAC,
			soundRate:   av.SOUND_44Khz,
			soundSize:   av.SOUND_16BIT,
			soundType:   av.SOUND_MONO,

			aacPacketType: av.AAC_RAW,
		},
	}
	tagBuffer := muxerTagData(tag)
	index := 0
	buffer = make([]byte, len(tagBuffer)+len(src))
	copy(buffer[index:], tagBuffer)
	index += len(tagBuffer)
	copy(buffer[index:], src)
	index += len(src)
	return buffer[:index]
}

//MuxerTagData 打包tag头和数据部分，在用rtmp协议发送时，tag头只包含了mediaTag，没有flvTag数据
//应该时flvTag这部分功能被chunk的功能替代了，不用flvTag也可以知道一个完整的帧，如果打包成flv文件时，
//flvTag不能省略
func muxerTagData(tag *Tag) []byte {
	n := 0
	buffer := make([]byte, 5) //16是按最大的长度来计算，aac有16个字节长度头
	if tag.flvt.fType == av.TAG_VIDEO {
		buffer[n] = (tag.mediat.frameType << 4) | (tag.mediat.codecID & 0x0F) //帧类型 4bit 编码id 4bit
		n++
		if tag.mediat.codecID == av.VIDEO_H264 {
			//如果是h264,有额外的封装
			utils.PutU8(buffer[n:], tag.mediat.avcPacketType) //AVCPacketType 8bit
			n++
			utils.PutU24BE(buffer[n:], uint32(tag.mediat.compositionTime)) //CompositionTime 24bit
			n += 3
		}
	} else if tag.flvt.fType == av.TAG_AUDIO {
		//音频格式 4bit
		//采样率 2bit
		//采样长度 1bit
		//音频类型 1bit
		buffer[n] = (tag.mediat.soundFormat << 4) | (tag.mediat.soundRate << 2 & 0x0C) |
			(tag.mediat.soundSize << 1 & 0x02) | (tag.mediat.soundType & 0x01)
		n++
		if tag.mediat.soundFormat == av.SOUND_AAC {
			utils.PutU8(buffer[n:], tag.mediat.aacPacketType) //AACPacketType
			n++
		}
	}
	//剩下的都认为是脚本tag，直接写入数据
	return buffer[:n]
}
