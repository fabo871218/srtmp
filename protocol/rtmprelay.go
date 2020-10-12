package protocol

import (
	"bytes"
	"fmt"
	"io"

	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol/amf"
	"github.com/fabo871218/srtmp/protocol/core"
)

var (
	STOP_CTRL = "RTMPRELAY_STOP"
)

type RtmpRelay struct {
	PlayUrl              string
	PublishUrl           string
	cs_chan              chan *core.ChunkStream
	sndctrl_chan         chan string
	connectPlayClient    *core.ConnClient
	connectPublishClient *core.ConnClient
	startflag            bool
	logger               logger.Logger
}

func NewRtmpRelay(playurl *string, publishurl *string) *RtmpRelay {
	return &RtmpRelay{
		PlayUrl:              *playurl,
		PublishUrl:           *publishurl,
		cs_chan:              make(chan *core.ChunkStream, 500),
		sndctrl_chan:         make(chan string),
		connectPlayClient:    nil,
		connectPublishClient: nil,
		startflag:            false,
	}
}

func (self *RtmpRelay) rcvPlayChunkStream() {
	fmt.Printf("rcvPlayRtmpMediaPacket connectClient.Read...\n")
	for {
		if self.startflag == false {
			self.connectPlayClient.Close()
			fmt.Printf("rcvPlayChunkStream close: playurl=%s, publishurl=%s\n", self.PlayUrl, self.PublishUrl)
			break
		}
		rc, err := self.connectPlayClient.Read()

		if err != nil && err == io.EOF {
			break
		}
		//glog.Infof("connectPlayClient.Read return rc.TypeID=%v length=%d, err=%v", rc.TypeID, len(rc.Data), err)
		switch rc.TypeID {
		case 20, 17:
			r := bytes.NewReader(rc.Data)
			vs, err := self.connectPlayClient.DecodeBatch(r, amf.AMF0)

			fmt.Printf("rcvPlayRtmpMediaPacket: vs=%v, err=%v\n", vs, err)
		case 18:
			fmt.Printf("rcvPlayRtmpMediaPacket: metadata....\n")
		case 8, 9:
			self.cs_chan <- rc
		}
	}
}

func (self *RtmpRelay) sendPublishChunkStream() {
	for {
		select {
		case rc := <-self.cs_chan:
			//glog.Infof("sendPublishChunkStream: rc.TypeID=%v length=%d", rc.TypeID, len(rc.Data))
			self.connectPublishClient.Write(rc)
		case ctrlcmd := <-self.sndctrl_chan:
			if ctrlcmd == STOP_CTRL {
				self.connectPublishClient.Close()
				fmt.Printf("sendPublishChunkStream close: playurl=%s, publishurl=%s\n", self.PlayUrl, self.PublishUrl)
				break
			}
		}
	}
}

//Start ...
func (self *RtmpRelay) Start() error {
	if self.startflag {
		return fmt.Errorf("The rtmprelay already started, playurl=%s, publishurl=%s", self.PlayUrl, self.PublishUrl)
	}

	self.connectPlayClient = core.NewConnClient(self.logger)
	self.connectPublishClient = core.NewConnClient(self.logger)

	self.logger.Debugf("Play server addr:%s starting....", self.PlayUrl)
	err := self.connectPlayClient.Start(self.PlayUrl, "play")
	if err != nil {
		fmt.Printf("connectPlayClient.Start url=%v error\n", self.PlayUrl)
		return err
	}

	fmt.Printf("publish server addr:%v starting....\n", self.PublishUrl)
	err = self.connectPublishClient.Start(self.PublishUrl, "publish")
	if err != nil {
		fmt.Printf("connectPublishClient.Start url=%v error\n", self.PublishUrl)
		self.connectPlayClient.Close()
		return err
	}

	self.startflag = true
	go self.rcvPlayChunkStream()
	go self.sendPublishChunkStream()

	return nil
}

func (self *RtmpRelay) Stop() {
	if !self.startflag {
		fmt.Printf("The rtmprelay already stoped, playurl=%s, publishurl=%s\n", self.PlayUrl, self.PublishUrl)
		return
	}

	self.startflag = false
	self.sndctrl_chan <- STOP_CTRL

}
