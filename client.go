package srtmp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/media/h264"
	"github.com/fabo871218/srtmp/protocol/amf"
	"github.com/fabo871218/srtmp/protocol/core"
)

//RtmpClient ...
type RtmpClient struct {
	packetChan      chan *av.Packet
	conn            *core.ConnClient
	onPacketReceive func(*av.Packet)
	onClosed        func()
	isPublish       bool
	videoFirst      bool //first packet to send
	audioFirst      bool
	demuxer         *flv.Demuxer
	logger          logger.Logger
}

//NewRtmpClient comment
func NewRtmpClient(log logger.Logger) *RtmpClient {
	return &RtmpClient{
		packetChan: make(chan *av.Packet, 16),
		videoFirst: true,
		audioFirst: true,
		demuxer:    flv.NewDemuxer(),
		logger:     log,
	}
}

//OpenPublish comment
func (c *RtmpClient) OpenPublish(URL string) (err error) {
	c.conn = core.NewConnClient(c.logger)
	if err = c.conn.Start(URL, "publish"); err != nil {
		return
	}

	c.isPublish = true
	return
}

//OpenPlay comment
func (c *RtmpClient) OpenPlay(URL string, onPacketReceive func(*av.Packet), onClosed func()) (err error) {
	c.conn = core.NewConnClient(c.logger)
	if err = c.conn.Start(URL, "play"); err != nil {
		return
	}

	c.onPacketReceive = onPacketReceive
	c.onClosed = onClosed
	go c.streamPlayProc()
	return
}

//Close 关闭连接，并回调onClosed
func (c *RtmpClient) Close() error {
	c.conn.Close()
	if c.onClosed != nil {
		c.onClosed()
	}
	return nil
}

//SendPacket todo
func (c *RtmpClient) SendPacket(pkt *av.Packet) error {
	if !c.isPublish {
		return fmt.Errorf("It is not publish mode")
	}

	switch pkt.PacketType {
	case av.PacketTypeAudio:
		ah, ok := pkt.Header.(av.AudioPacketHeader)
		if ok == false {
			return fmt.Errorf("audio pkt.Header should be av.AudioPacketHeader")
		}
		if c.audioFirst {
			if ah.SoundFormat == av.SOUND_AAC {
				//如果音频是aac，需要先发送aac sequence header
				sequencePkt := &av.Packet{
					PacketType: av.PacketTypeAudio,
					Data:       flv.NewAACSequenceHeader(ah),
					TimeStamp:  pkt.TimeStamp,
				}

				if err := c.sendPacket(sequencePkt); err != nil {
					return fmt.Errorf("send aac sequence header failed. %v", err)
				}
			}

			c.audioFirst = false
		}

		pkt.Data = flv.NewAACData(ah, pkt.Data, pkt.TimeStamp)
		if err := c.sendPacket(pkt); err != nil {
			return fmt.Errorf("send packet failed, %v", err)
		}
	case av.PacketTypeVideo:
		if c.videoFirst {
			var sps, pps []byte
			nalus := h264.ParseNalus(pkt.Data)
			for _, nalu := range nalus {
				if naluType := nalu[0] & 0x1F; naluType == 7 {
					sps = nalu
				} else if naluType == 8 {
					pps = nalu
				}
			}

			if sps == nil || pps == nil {
				fmt.Printf("sps and pps needed for first packet\n")
				return nil
			}
			//send flv sequence header
			sequencePkt := &av.Packet{
				PacketType: av.PacketTypeVideo,
				Data:       flv.NewAVCSequenceHeader(sps, pps, pkt.TimeStamp),
				TimeStamp:  pkt.TimeStamp,
			}

			if err := c.sendPacket(sequencePkt); err != nil {
				return fmt.Errorf("send flv sequence header failed, %v", err)
			}
			c.videoFirst = false
		}

		pkt.Data = flv.NewAVCNaluData(pkt.Data, pkt.TimeStamp)
		if err := c.sendPacket(pkt); err != nil {
			return fmt.Errorf("send packet failed, %v", err)
		}
	default:
		return fmt.Errorf("packet type is not video and audio")
	}
	return nil
}

//发送数据包
func (c *RtmpClient) sendPacket(pkt *av.Packet) error {
	var cs core.ChunkStream
	cs.Data = pkt.Data
	cs.Length = uint32(len(pkt.Data))
	cs.StreamID = c.conn.GetStreamID()
	cs.Timestamp = pkt.TimeStamp

	switch pkt.PacketType {
	case av.PacketTypeVideo:
		cs.TypeID = av.TAG_VIDEO
	case av.PacketTypeAudio:
		cs.TypeID = av.TAG_AUDIO
	case av.PacketTypeMetadata:
		cs.TypeID = av.TAG_SCRIPTDATAAMF0
	}

	if err := c.conn.Write(&cs); err != nil {
		return err
	} else if err := c.conn.Flush(); err != nil {
		return err
	}
	return nil
}

func (c *RtmpClient) handleVideoAudio(cs *core.ChunkStream) (err error) {
	var pkt av.Packet
	pkt.Data = cs.Data
	pkt.StreamID = cs.StreamID
	pkt.TimeStamp = cs.Timestamp
	if cs.TypeID == av.TAG_AUDIO {
		pkt.PacketType = av.PacketTypeAudio
	} else if cs.TypeID == av.TAG_VIDEO {
		c.logger.Debugf("Debug.... %s", hex.EncodeToString(cs.Data[:20]))
		pkt.PacketType = av.PacketTypeVideo
	}
	if err = c.demuxer.Demux(&pkt); err != nil {
		return fmt.Errorf("Demux failed, %v", err)
	}

	if pkt.PacketType == av.PacketTypeAudio {
		//如果是音频数据，直接回调出去
		c.onPacketReceive(&pkt)
		return nil
	}
	//如果是视频数据，需要区分是不是sequence header，如果是sequence，需要解析出sps和pps信息
	vh, ok := pkt.Header.(av.VideoPacketHeader)
	if !ok {
		return fmt.Errorf("cannot convert from pkt.Header to av.VideoPacketHeader")
	}

	if vh.CodecID != av.VIDEO_H264 {
		return fmt.Errorf("code id:%d do not support", vh.CodecID)
	}
	//判断是不是sequence header
	if vh.FrameType == av.FRAME_KEY && vh.AVCPacketType == av.AVC_SEQHDR {
		spss, ppss, err := flv.ParseAVCSequenceHeader(pkt.Data)
		if err != nil {
			return fmt.Errorf("parse avc sequence header failed, %v", err)
		}
		//如果解析到多个sps和pps，只返回第一个sps和pps
		if len(spss) > 0 {
			pkt.Data = spss[0]
			c.onPacketReceive(&pkt)
		}
		if len(ppss) > 0 {
			pkt.Data = ppss[0]
			c.onPacketReceive(&pkt)
		}
		return nil
	}
	//解析后的数据格式为 4字节长度+nalue数据+4字节长度+nalu数据。。。
	//解析出所以的nalu数据
	index := 0
	naluData := pkt.Data
	for {
		remain := len(naluData[index:])
		if remain < 4 {
			if remain != 0 {
				c.logger.Warnf("Invalid data length, remain:%d", remain)
			}
			return nil
		}

		length := binary.BigEndian.Uint32(naluData[index:])
		if length > uint32(remain-4) {
			return fmt.Errorf("invalid data length:%d remain:%d", length, remain-4)
		}
		index += 4
		pkt.Data = naluData[index : index+int(length)]
		index += int(length)
		c.onPacketReceive(&pkt)
	}
}

func (c *RtmpClient) handleMetadata(cs *core.ChunkStream) (err error) {
	var values []interface{}
	r := bytes.NewReader(cs.Data)
	if cs.TypeID == av.TAG_SCRIPTDATAAMF0 {
		values, err = c.conn.DecodeBatch(r, amf.AMF0)
	} else if cs.TypeID == av.TAG_SCRIPTDATAAMF3 {
		values, err = c.conn.DecodeBatch(r, amf.AMF3)
	}
	if err != nil && err != io.EOF {
		return fmt.Errorf("decode metadata failed, %v", err)
	}

	for _, v := range values {
		switch v.(type) {
		case string:
			if v.(string) == "onMetadata" {
				//说明该信息是描述视频信息的元数据，可以从afm.Object中获取到相印的属性值
			}
		case amf.Object:
			for k, v1 := range v.(amf.Object) {
				c.logger.Debugf("key:%s v:%v", k, v1)
			}
		default: //其他的忽略不处理
		}
	}
	return nil
}

//处理命令消息
func (c *RtmpClient) handleCommand(cs *core.ChunkStream) (err error) {
	var values []interface{}
	r := bytes.NewReader(cs.Data)
	if cs.TypeID == 20 {
		values, err = c.conn.DecodeBatch(r, amf.AMF0)
	} else if cs.TypeID == 17 {
		values, err = c.conn.DecodeBatch(r, amf.AMF3)
	}
	if err != nil && err != io.EOF {
		return fmt.Errorf("Decode amf failed, %v", err)
	}
	for k, v := range values {
		c.logger.Tracef("k:%d v:%v", k, v)
	}
	return nil
}

func (c *RtmpClient) streamPlayProc() {
	defer c.Close()
	for {
		cs, err := c.conn.Read()
		if err != nil {
			c.logger.Errorf("Read chunk stream failed, %s", err.Error())
			break
		}

		switch cs.TypeID {
		case av.TAG_AUDIO, av.TAG_VIDEO:
			c.logger.Debugf("Receive a media data..... type:%d len:%d", cs.TypeID, len(cs.Data))
			if err := c.handleVideoAudio(cs); err != nil {
				c.logger.Errorf("handle media data failed, %v", err)
			}
		case av.TAG_SCRIPTDATAAMF0, av.TAG_SCRIPTDATAAMF3:
			c.logger.Debug("Receive a scriptdata.....")
			if err := c.handleMetadata(cs); err != nil {
				c.logger.Errorf("handle metadata failed, %v", err)
			}
		case 17, 20:
			c.logger.Debug("Receive a command message.....")
			if err := c.handleCommand(cs); err != nil {
				c.logger.Errorf("handle command failed, %v", err)
			}
		default:
			c.logger.Errorf("Unsupport type id:%d", cs.TypeID)
			continue
		}
	}
}
