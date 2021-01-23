package protocol

import (
	"errors"
	"fmt"
	"sync"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/configure"
	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol/core"
)

type StaticPush struct {
	RtmpUrl       string
	packet_chan   chan *av.Packet
	sndctrl_chan  chan string
	connectClient *core.ConnClient
	startflag     bool
	logger        logger.Logger
}

var G_StaticPushMap = make(map[string](*StaticPush))
var g_MapLock = new(sync.RWMutex)

var (
	STATIC_RELAY_STOP_CTRL = "STATIC_RTMPRELAY_STOP"
)

func GetStaticPushList(appname string) ([]string, error) {
	pushurlList, ok := configure.GetStaticPushUrlList(appname)

	if !ok {
		return nil, errors.New("no static push url")
	}

	return pushurlList, nil
}

func GetAndCreateStaticPushObject(rtmpurl string) *StaticPush {
	g_MapLock.RLock()
	staticpush, ok := G_StaticPushMap[rtmpurl]
	fmt.Printf("GetAndCreateStaticPushObject: %s, return %v\n", rtmpurl, ok)
	if !ok {
		g_MapLock.RUnlock()
		newStaticpush := NewStaticPush(rtmpurl)

		g_MapLock.Lock()
		G_StaticPushMap[rtmpurl] = newStaticpush
		g_MapLock.Unlock()

		return newStaticpush
	}
	g_MapLock.RUnlock()

	return staticpush
}

func GetStaticPushObject(rtmpurl string) (*StaticPush, error) {
	g_MapLock.RLock()
	if staticpush, ok := G_StaticPushMap[rtmpurl]; ok {
		g_MapLock.RUnlock()
		return staticpush, nil
	}
	g_MapLock.RUnlock()

	return nil, errors.New(fmt.Sprintf("G_StaticPushMap[%s] not exist...."))
}

func ReleaseStaticPushObject(rtmpurl string) {
	g_MapLock.RLock()
	if _, ok := G_StaticPushMap[rtmpurl]; ok {
		g_MapLock.RUnlock()

		fmt.Printf("ReleaseStaticPushObject %s ok\n", rtmpurl)
		g_MapLock.Lock()
		delete(G_StaticPushMap, rtmpurl)
		g_MapLock.Unlock()
	} else {
		g_MapLock.RUnlock()
		fmt.Printf("ReleaseStaticPushObject: not find %s\n", rtmpurl)
	}
}

func NewStaticPush(rtmpurl string) *StaticPush {
	return &StaticPush{
		RtmpUrl:       rtmpurl,
		packet_chan:   make(chan *av.Packet, 500),
		sndctrl_chan:  make(chan string),
		connectClient: nil,
		startflag:     false,
	}
}

func (self *StaticPush) Start() error {
	if self.startflag {
		return errors.New(fmt.Sprintf("StaticPush already start %s", self.RtmpUrl))
	}

	self.connectClient = core.NewConnClient(self.logger)

	fmt.Printf("static publish server addr:%v starting....\n", self.RtmpUrl)
	err := self.connectClient.Start(self.RtmpUrl, "publish")
	if err != nil {
		fmt.Printf("connectClient.Start url=%v error\n", self.RtmpUrl)
		return err
	}
	fmt.Printf("static publish server addr:%v started, streamid=%d\n", self.RtmpUrl, self.connectClient.GetStreamID())
	go self.HandleAvPacket()

	self.startflag = true
	return nil
}

func (self *StaticPush) Stop() {
	if !self.startflag {
		return
	}

	fmt.Printf("StaticPush Stop: %s\n", self.RtmpUrl)
	self.sndctrl_chan <- STATIC_RELAY_STOP_CTRL
	self.startflag = false
}

func (self *StaticPush) WriteAvPacket(packet *av.Packet) {
	if !self.startflag {
		return
	}

	self.packet_chan <- packet
}

func (self *StaticPush) sendPacket(p *av.Packet) {
	if !self.startflag {
		return
	}
	var cs core.ChunkStream

	cs.Data = p.Data
	cs.Length = uint32(len(p.Data))
	cs.StreamID = self.connectClient.GetStreamID()
	cs.Timestamp = p.TimeStamp
	//cs.Timestamp += v.BaseTimeStamp()

	//glog.Infof("Static sendPacket: rtmpurl=%s, length=%d, streamid=%d",
	//	self.RtmpUrl, len(p.Data), cs.StreamID)
	switch p.PacketType {
	case av.PacketTypeVideo:
		cs.TypeID = av.TAG_VIDEO
	case av.PacketTypeAudio:
		cs.TypeID = av.TAG_AUDIO
	case av.PacketTypeMetadata:
		cs.TypeID = av.TAG_SCRIPTDATAAMF0
	default:
	}
	self.connectClient.Write(&cs)
}

func (self *StaticPush) HandleAvPacket() {
	if !self.IsStart() {
		fmt.Printf("static push %s not started\n", self.RtmpUrl)
		return
	}

	for {
		select {
		case packet := <-self.packet_chan:
			self.sendPacket(packet)
		case ctrlcmd := <-self.sndctrl_chan:
			if ctrlcmd == STATIC_RELAY_STOP_CTRL {
				self.connectClient.Close()
				fmt.Printf("Static HandleAvPacket close: publishurl=%s\n", self.RtmpUrl)
				break
			}
		}
	}
}

func (self *StaticPush) IsStart() bool {
	return self.startflag
}
