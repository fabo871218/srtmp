package srtmp

import (
	"fmt"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/media/h264"
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

//Close comment
func (c *RtmpClient) Close() error {
	c.conn.Close()
	return nil
}

//SendPacket todo
func (c *RtmpClient) SendPacket(pkt *av.Packet) error {
	if !c.isPublish {
		return fmt.Errorf("It is not publish mode")
	}

	if pkt.IsVideo {
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
				IsVideo:   true,
				Data:      flv.NewAVCSequenceHeader(sps, pps, pkt.TimeStamp),
				TimeStamp: pkt.TimeStamp,
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
	} else if pkt.IsAudio {
		ah, ok := pkt.Header.(av.AudioPacketHeader)
		if ok == false {
			return fmt.Errorf("audio pkt.Header should be av.AudioPacketHeader")
		}
		if c.audioFirst {
			if ah.SoundFormat() == av.SOUND_AAC {
				//如果音频是aac，需要先发送aac sequence header
				sequencePkt := &av.Packet{
					IsAudio:   true,
					Data:      flv.NewAACSequenceHeader(ah),
					TimeStamp: pkt.TimeStamp,
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
	} else {
		return fmt.Errorf("packet type is not video and audio")
	}
	return nil
}

func (c *RtmpClient) sendPacket(pkt *av.Packet) error {
	var cs core.ChunkStream
	cs.Data = pkt.Data
	cs.Length = uint32(len(pkt.Data))
	cs.StreamID = c.conn.GetStreamId()
	cs.Timestamp = pkt.TimeStamp

	if pkt.IsVideo {
		cs.TypeID = av.TAG_VIDEO
	} else {
		if pkt.IsMetadata {
			cs.TypeID = av.TAG_SCRIPTDATAAMF0
		} else {
			cs.TypeID = av.TAG_AUDIO
		}
	}

	if err := c.conn.Write(cs); err != nil {
		return err
	} else if err := c.conn.Flush(); err != nil {
		return err
	}
	return nil
}

func (c *RtmpClient) streamPlayProc() {
	defer c.onClosed()
	var cs core.ChunkStream
	for {
		if err := c.conn.Read(&cs); err != nil {
			fmt.Printf("read chunk stream failed, %v", err)
			break
		}

		var pkt av.Packet
		pkt.IsAudio = cs.TypeID == av.TAG_AUDIO
		pkt.IsVideo = cs.TypeID == av.TAG_VIDEO
		pkt.IsMetadata = cs.TypeID == av.TAG_SCRIPTDATAAMF0 || cs.TypeID == av.TAG_SCRIPTDATAAMF3
		pkt.StreamID = cs.StreamID
		pkt.Data = cs.Data
		pkt.TimeStamp = cs.Timestamp
		c.demuxer.DemuxH(&pkt)

		c.onPacketReceive(&pkt)
	}
}