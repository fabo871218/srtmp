package protocol

import (
	"fmt"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/media/h264"
	"github.com/fabo871218/srtmp/protocol/core"
)

type RtmpClient struct {
	packetChan      chan *av.Packet
	conn            *core.ConnClient
	onPacketReceive func(av.Packet)
	isPublish       bool
	isFirstPacket   bool //first packet to send
}

func NewRtmpClient() *RtmpClient {
	return &RtmpClient{
		packetChan:    make(chan *av.Packet, 16),
		isFirstPacket: true,
	}
}

func (c *RtmpClient) OpenPublish(URL string) (err error) {
	c.conn = core.NewConnClient()
	if err = c.conn.Start(URL, "publish"); err != nil {
		return
	}

	c.isPublish = true
	return
}

func (c *RtmpClient) OpenPlay(URL string, onPacketReceive func(av.Packet)) (err error) {
	c.conn = core.NewConnClient()
	if err = c.conn.Start(URL, "play"); err != nil {
		return
	}

	c.onPacketReceive = onPacketReceive
	go c.streamPlayProc()
	return
}

func (c *RtmpClient) Close() error {
	c.conn.Close()
	return nil
}

//SendPacket todo
func (c *RtmpClient) SendPacket(pkt *av.Packet) error {
	if !c.isPublish {
		return fmt.Errorf("It is not publish mode")
	}

	if c.isFirstPacket {
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
			Data:      flv.NewAACSequenceHeader(sps, pps, pkt.TimeStamp),
			TimeStamp: pkt.TimeStamp,
		}

		if err := c.sendPacket(sequencePkt); err != nil {
			return fmt.Errorf("send flv sequence header failed, %v", err)
		}
		c.isFirstPacket = false
	}

	pkt.Data = flv.NewAACNalu(pkt.Data, pkt.TimeStamp)
	if err := c.sendPacket(pkt); err != nil {
		return fmt.Errorf("send packet failed, %v", err)
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

}
