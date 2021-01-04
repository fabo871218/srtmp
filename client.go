package srtmp

import (
	"bytes"
	"encoding/binary"
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

// SendPacket 发送数据包
func (c *RtmpClient) SendPacket(pkt *av.Packet) error {
	if !c.isPublish {
		return fmt.Errorf("It is not publish mode")
	}

	switch pkt.PacketType {
	case av.PacketTypeAudio:
		return c.sendAudioPacket(pkt)
	case av.PacketTypeVideo:
		return c.sendVideoPacket(pkt)
	case av.PacketTypeMetadata:
		return c.sendMetaPacket(pkt)
	default:
		return fmt.Errorf("Unknow packet type:%d", pkt.PacketType)
	}
}

func (c *RtmpClient) sendAudioPacket(pkt *av.Packet) error {
	var err error
	if pkt.AHeader.SoundFormat == av.SOUND_AAC && c.audioFirst {
		//如果音频是aac，需要先发送aac sequence header
		sequencePkt := &av.Packet{
			PacketType: av.PacketTypeAudio,
			Data:       flv.NewAACSequenceHeader(pkt.AHeader),
			TimeStamp:  pkt.TimeStamp,
		}
		if err = c.sendPacketData(sequencePkt.Data, sequencePkt.TimeStamp,
			av.PacketTypeAudio); err != nil {
			return fmt.Errorf("send aac sequence header failed. %v", err)
		}
		c.audioFirst = false
	}

	if pkt.Data, err = flv.PackAudioData(&pkt.AHeader, pkt.StreamID, pkt.Data,
		pkt.TimeStamp); err != nil {
		return fmt.Errorf("Pack audio failed. %v", err)
	}
	if err = c.sendPacketData(pkt.Data, pkt.TimeStamp, av.PacketTypeAudio); err != nil {
		return fmt.Errorf("send packet failed, %v", err)
	}
	return nil
}

func (c *RtmpClient) sendVideoPacket(pkt *av.Packet) error {
	var err error
	if pkt.VHeader.CodecID == av.VIDEO_H264 {
		// 如果是h264，第一帧要发送sequence header
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
				c.logger.Warn("sps and pps need for first packet.")
				return nil
			}
			//send flv sequence header
			sequencePkt := &av.Packet{
				PacketType: av.PacketTypeVideo,
				Data:       flv.NewAVCSequenceHeader(sps, pps, pkt.TimeStamp),
				TimeStamp:  pkt.TimeStamp,
			}

			if err = c.sendPacketData(sequencePkt.Data, sequencePkt.TimeStamp,
				av.PacketTypeVideo); err != nil {
				return fmt.Errorf("send flv sequence header failed, %v", err)
			}
			c.videoFirst = false
		}
	}
	if pkt.Data, err = flv.PackVideoData(&pkt.VHeader, pkt.StreamID, pkt.Data,
		pkt.TimeStamp); err != nil {
		return fmt.Errorf("Pack video failed, %v", err)
	}
	if err = c.sendPacketData(pkt.Data, pkt.TimeStamp, av.PacketTypeVideo); err != nil {
		return fmt.Errorf("send packet failed, %v", err)
	}
	return nil
}

func (c *RtmpClient) sendMetaPacket(pkt *av.Packet) error {
	return fmt.Errorf("Mata data unsupport")
}

func (c *RtmpClient) sendPacketData(data []byte, timestamp uint32, packetType int) error {
	if len(data) == 0 {
		return fmt.Errorf("data length is zero")
	}

	var typeID uint32
	switch packetType {
	case av.PacketTypeVideo:
		typeID = av.TAG_VIDEO
	case av.PacketTypeAudio:
		typeID = av.TAG_AUDIO
	case av.PacketTypeMetadata:
		typeID = av.TAG_SCRIPTDATAAMF0
	default:
		return fmt.Errorf("Unsupport packet type:%d", packetType)
	}

	// todo 其他的字段值是否有效
	cs := core.ChunkStream{
		Data:      data,
		Length:    uint32(len(data)),
		StreamID:  c.conn.GetStreamID(),
		Timestamp: timestamp,
		TypeID:    typeID,
	}

	if err := c.conn.Write(&cs); err != nil {
		return err
	} else if err := c.conn.Flush(); err != nil {
		return err
	}
	return nil
}

// 从ChunkStream中解析音频和视频数据
func (c *RtmpClient) handleVideoAudio(cs *core.ChunkStream) error {
	var pktType uint32
	switch cs.TypeID {
	case av.TAG_VIDEO:
		pktType = av.PacketTypeVideo
	case av.TAG_AUDIO:
		pktType = av.PacketTypeAudio
	case av.TAG_SCRIPTDATAAMF0, av.TAG_SCRIPTDATAAMF3:
		pktType = av.PacketTypeMetadata
	default:
		return fmt.Errorf("Unknow chunk type:%d", cs.TypeID)
	}

	var err error
	pkt := av.Packet{
		Data:       cs.Data,
		StreamID:   cs.StreamID,
		TimeStamp:  cs.Timestamp,
		PacketType: pktType,
	}
	if err = c.demuxer.Demux(&pkt); err != nil {
		return fmt.Errorf("Demux failed, %v", err)
	}

	switch pkt.PacketType {
	case av.PacketTypeAudio: //处理音频数据
		c.onPacketReceive(&pkt)
	case av.PacketTypeVideo: //处理视频数据
		switch pkt.VHeader.CodecID {
		case av.VIDEO_H264:
			// 如果是h264的sequence header，需要解析出sps和pps
			if pkt.VHeader.FrameType == av.FRAME_KEY && pkt.VHeader.AVCPacketType == av.AVC_SEQHDR {
				spss, ppss, err := flv.ParseAVCSequenceHeader(pkt.Data)
				if err != nil {
					return fmt.Errorf("Parse avc sequence header failed, %v", err)
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
		default:
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
	case av.TAG_SCRIPTDATAAMF0, av.TAG_SCRIPTDATAAMF3:
		return fmt.Errorf("TODO")
	default:
		return fmt.Errorf("unknow chunk stream type:%d", cs.TypeID)
	}
	return nil
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
