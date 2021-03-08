package srtmp

import (
	"errors"
	"fmt"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/media/h264"
	"github.com/fabo871218/srtmp/protocol/amf"
)

const (
	//TrackTypeVideo 音频轨道
	TrackTypeVideo = 0
	//TrackTypeAudio 视频轨道
	TrackTypeAudio = 1
	// TrackTypeMateData 信令轨道
	TrackTypeMateData = 2

	// MessageTypeVideo 视频消息
	MessageTypeVideo = 0
	// MessageTypeAudio 音频消息
	MessageTypeAudio = 1
	// MessageTypeMateData 脚本消息
	MessageTypeMateData = 2
)

// StreamMessage 主要包括audio消息，video消息
type StreamMessage struct {
	MessageType uint32
	Pts         uint32
	Dts         uint32
	Payload     []byte
	// extend 传递音视频格式信息和decode后的script
	extend interface{}
}

// VideoTrackInfo ...
type VideoTrackInfo struct {
	CodecID uint32
	Height  uint32
	Width   uint32
}

// AudioTrackInfo ...
type AudioTrackInfo struct {
	CodecID    uint32
	SampleRate uint32
	DataBit    uint32
	Channels   uint32
}

// StreamTrack 一个流传输通道，通常只支持一路音频，一路视频和一路脚本
// 使用同一个streamID
type StreamTrack struct {
	client             *RtmpClient
	video              *VideoTrackInfo
	audio              *AudioTrackInfo
	streamID           uint32
	mateData           amf.Object // 音视频信息的描述
	bfirstAudioMessage bool
	bfirstVideoMessage bool
}

// VideoInfo 返回该流中的视频信息,
// 如果是play，该接口必须等接收到视频消息后才能保证返回视频格式
func (t *StreamTrack) VideoInfo() *VideoTrackInfo {
	return t.video
}

// AudioInfo 返回该stream track中的音频信息，
// 如果是play，该接口必须等接收到音频消息后才能保证返回的视频格式
func (t *StreamTrack) AudioInfo() *AudioTrackInfo {
	return t.audio
}

// ReadMessage 读取一个消息
func (t *StreamTrack) ReadMessage() (*StreamMessage, error) {
	for {
		msg, err := t.client.readMessage(t.streamID)
		if err != nil {
			return nil, fmt.Errorf("read message failed, %v", err)
		}

		switch msg.MessageType {
		case MessageTypeAudio:
			if t.audio == nil {
				if header, ok := msg.extend.(*av.AudioPacketHeader); ok {
					t.audio = &AudioTrackInfo{
						// todo 类型取值是否一直
						CodecID:    uint32(header.SoundFormat),
						SampleRate: uint32(header.SoundRate),
						DataBit:    uint32(header.SoundSize),
						Channels:   uint32(header.SoundType),
					}
				}
			}
		case MessageTypeVideo:
			if t.video == nil {
				if header, ok := msg.extend.(*av.VideoPacketHeader); ok {
					t.video = &VideoTrackInfo{
						CodecID: uint32(header.CodecID),
					}

					if t.mateData != nil {
						for key, value := range t.mateData {
							switch key {
							case "width":
								if number, ok := value.(float64); ok {
									t.video.Width = uint32(number)
								}
							case "height":
								if number, ok := value.(float64); ok {
									t.video.Height = uint32(number)
								}
							default:
								// TODO 其他的信息暂时不获取
							}
						}
					}
				}
			}
		case MessageTypeMateData:
			if vs, ok := msg.extend.([]interface{}); ok {
				if len(vs) == 0 {
					continue
				}

				if value, ok := vs[0].(string); ok && value == "onMetaData" {
					// 从onMateData中获取到相关的信息
					if len(vs) >= 2 {
						if objmap, ok := vs[1].(amf.Object); ok {
							t.mateData = objmap
						}
					}
				}
			}
			continue
		}
		return msg, nil
	}
}

// WriteMessage 发送一个消息
func (t *StreamTrack) WriteMessage(msg *StreamMessage) error {
	switch msg.MessageType {
	case MessageTypeVideo:
		return t.writeVideoMessage(msg)
	case MessageTypeAudio:
		return t.writeAudioMessage(msg)
	case MessageTypeMateData:
		return t.writeMateDataMessage(msg)
	default:
		return fmt.Errorf("unknown message type:%d", msg.MessageType)
	}
}

func newStreamTrack(streamID uint32, audio *AudioTrackInfo, video *VideoTrackInfo, c *RtmpClient) *StreamTrack {
	return &StreamTrack{
		client:             c,
		streamID:           streamID,
		audio:              audio,
		video:              video,
		bfirstAudioMessage: true,
		bfirstVideoMessage: true,
	}
}

func (t *StreamTrack) writeVideoMessage(msg *StreamMessage) error {
	if t.video == nil {
		return errors.New("video info is nil")
	}

	if t.bfirstVideoMessage {
		// 如果是第一个包，且是h264编码，需要发送avc header
		if t.video.CodecID == av.VideoH264 {
			var sps, pps []byte
			nalus := h264.ParseNalus(msg.Payload)
			for _, nalu := range nalus {
				if naluType := nalu[0] & 0x1F; naluType == 7 {
					sps = nalu
				} else if naluType == 8 {
					pps = nalu
				}
			}

			if sps == nil || pps == nil {
				//t.logger.Warn("sps and pps need for first packet.")
				return nil
			}
			//send flv sequence header
			sequenceData := flv.NewAVCSequenceHeader(sps, pps, msg.Pts, false)
			sequenceMsg := StreamMessage{
				MessageType: MessageTypeVideo,
				Payload:     sequenceData,
				Pts:         msg.Pts,
				Dts:         msg.Dts,
			}
			if err := t.client.sendMessage(t.streamID, &sequenceMsg); err != nil {
				return fmt.Errorf("send chunk failed, %v", err)
			}
		}
		t.bfirstVideoMessage = false
	}

	vh := av.VideoPacketHeader{
		FrameType:       av.FrameInter, // todo 这个值需不需要传递
		AVCPacketType:   av.AvcNALU,
		CodecID:         uint8(t.video.CodecID),
		CompositionTime: 0, // todo
	}
	videoData, err := flv.PackVideoData(&vh, false, t.streamID, msg.Payload, msg.Pts)
	if err != nil {
		return fmt.Errorf("flv.PackVideoData %v", err)
	}
	msg.Payload = videoData
	if err := t.client.sendMessage(t.streamID, msg); err != nil {
		return fmt.Errorf("send video chunk failed, %v", err)
	}
	return nil
}

func (t *StreamTrack) writeAudioMessage(msg *StreamMessage) error {
	if t.audio == nil {
		return errors.New("audio msg is nil")
	}

	if t.bfirstAudioMessage {
		if t.audio.CodecID == av.SOUND_AAC {
			ah := av.AudioPacketHeader{
				SoundFormat: uint8(t.audio.CodecID),
				SoundRate:   uint8(t.audio.SampleRate),
				SoundSize:   uint8(t.audio.DataBit),
				SoundType:   1, // for aac, always 1
			}
			sequenceData := flv.NewAACSequenceHeader(ah, false)
			sequenceMsg := StreamMessage{
				MessageType: MessageTypeAudio,
				Payload:     sequenceData,
				Dts:         msg.Dts,
				Pts:         msg.Pts,
			}

			if err := t.client.sendMessage(t.streamID, &sequenceMsg); err != nil {
				return fmt.Errorf("send sequence failed, %v", err)
			}
		}
		t.bfirstAudioMessage = false
	}
	ah := av.AudioPacketHeader{
		SoundFormat:   uint8(t.audio.CodecID),
		SoundRate:     uint8(t.audio.SampleRate),
		SoundSize:     uint8(t.audio.DataBit),
		SoundType:     1, // todo 要确定一下该值怎么填写
		AACPacketType: 0, // 该字段在发送的时候可以忽略
	}
	videoData, err := flv.PackAudioData(&ah, false, t.streamID, msg.Payload, msg.Pts)
	if err != nil {
		return fmt.Errorf("pack audio data failed, %v", err)
	}
	msg.Payload = videoData
	if err := t.client.sendMessage(t.streamID, msg); err != nil {
		return fmt.Errorf("send chunk failed, %v", err)
	}
	return nil
}

func (t *StreamTrack) writeMateDataMessage(msg *StreamMessage) error {
	return fmt.Errorf("not support") // TODO 还不支持发送
}
