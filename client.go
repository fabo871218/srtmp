package srtmp

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol/amf"
	"github.com/fabo871218/srtmp/protocol/core"
)

// RtmpConnectState rtmp client 连接状态
type RtmpConnectState int

const (
	StateConnectSuccess = 0
	StateConnectFailed  = 1
	StateDisconnect     = 2
)

//RtmpClient ...
type RtmpClient struct {
	packetChan      chan *av.Packet
	conn            *core.ConnClient
	onPacketReceive func(*av.Packet)
	videoFirst      bool //first packet to send
	audioFirst      bool
	demuxer         *flv.Demuxer
	logger          logger.Logger

	streamIndex    uint32
	onStreamTrack  func(*StreamTrack)
	onConnectState func(RtmpConnectState, error)
	tracks         []*StreamTrack
	streamChans    map[int]chan *StreamMessage
	sendChan       chan *core.ChunkStream
	errChan        chan error
	closed         chan bool
}

//NewRtmpClient create a new rtmp client
func newRtmpClient(log logger.Logger) *RtmpClient {
	return &RtmpClient{
		packetChan:  make(chan *av.Packet, 16),
		videoFirst:  true,
		audioFirst:  true,
		demuxer:     flv.NewDemuxer(),
		logger:      log,
		streamIndex: 1, //stream message id,从1开始
		errChan:     make(chan error, 1),
		closed:      make(chan bool, 1),
		sendChan:    make(chan *core.ChunkStream, 8),
		streamChans: make(map[int]chan *StreamMessage),
	}
}

// OpenPublish 开始连接制定URL，并推送流
// 在调用OpenPublish之前，必须先调用AddStreamTrack
func (c *RtmpClient) OpenPublish(URL string, f func(RtmpConnectState, error)) error {
	c.onConnectState = f
	go func() {
		c.conn = core.NewConnClient(c.logger)
		if err := c.conn.Start(URL, "publish"); err != nil {
			c.onError(err)
			c.onConnectState(StateConnectFailed, fmt.Errorf("connect failed, %v", err))
			return
		}

		c.onConnectState(StateConnectSuccess, nil)
		go c.readloop()
		c.mainloop(true)
	}()
	return nil
}

// OpenPlay 根据url开始播放音视频流，在调用该接口之前，必须先调用OnStreamTrack
// 设置接收到streamtrack的回调
func (c *RtmpClient) OpenPlay(URL string, f func(RtmpConnectState, error)) error {
	c.onConnectState = f
	go func() {
		c.conn = core.NewConnClient(c.logger)
		if err := c.conn.Start(URL, "play"); err != nil {
			c.onError(err)
			c.onConnectState(StateConnectFailed, fmt.Errorf("connect failed, %v", err))
			return
		}

		c.onConnectState(StateConnectSuccess, nil)
		go c.readloop()
		c.mainloop(false)
	}()
	return nil
}

//Close 关闭连接
func (c *RtmpClient) Close() {
	select {
	case c.closed <- true:
	default:
	}
}

// OnStreamTrack 设置收到数据流的回调
func (c *RtmpClient) OnStreamTrack(f func(*StreamTrack)) {
	c.onStreamTrack = f
}

// AddStreamTrack 添加一个stream track，包含一路视频和一路音频
// 如果没有视频或音频，可以设置为nil
func (c *RtmpClient) AddStreamTrack(audio *AudioTrackInfo, video *VideoTrackInfo) (*StreamTrack, error) {
	track := newStreamTrack(c.streamIndex, audio, video, c)
	c.tracks = append(c.tracks, track)
	c.streamIndex++
	return track, nil
}

// 从对应的streamID中读取一个消息
func (c *RtmpClient) readMessage(streamID uint32) (*StreamMessage, error) {
	if outchan, ok := c.streamChans[int(streamID)]; ok {
		select {
		case chunk := <-outchan:
			return chunk, nil
		case err := <-c.errChan:
			c.onError(err)
			return nil, err
		}
	}
	return nil, fmt.Errorf("stream track:%d not found", streamID)
}

func (c *RtmpClient) sendMessage(streamID uint32, msg *StreamMessage) error {
	var cs *core.ChunkStream
	switch msg.MessageType {
	case MessageTypeAudio:
		cs = &core.ChunkStream{
			Data:      msg.Payload,
			Length:    uint32(len(msg.Payload)),
			StreamID:  streamID,
			Timestamp: msg.Pts,
			TypeID:    av.TAG_AUDIO,
		}
	case MessageTypeVideo:
		cs = &core.ChunkStream{
			Data:      msg.Payload,
			Length:    uint32(len(msg.Payload)),
			StreamID:  streamID,
			Timestamp: msg.Pts,
			TypeID:    av.TAG_VIDEO,
		}
	case MessageTypeMateData:
		// 因为payload无法保存要发送的脚本，所以通过extend传递
		if msg.extend == nil {
			return fmt.Errorf("extend is nil")
		}
		msgs, ok := msg.extend.([]interface{})
		if !ok {
			return fmt.Errorf("can not convert from extend to []interface{}")
		}
		if len(msgs) == 0 {
			return fmt.Errorf("no script msg find")
		}
		bw := &bytes.Buffer{}
		for _, msg := range msgs {
			if _, err := c.conn.Encode(bw, msg, amf.AMF0); err != nil {
				return fmt.Errorf("encode value failed, %v", err)
			}
		}
		sendBytes := bw.Bytes()
		cs = &core.ChunkStream{
			Data:      sendBytes,
			Length:    uint32(len(sendBytes)),
			StreamID:  streamID,
			Timestamp: msg.Pts,
			TypeID:    av.TAG_SCRIPTDATAAMF0,
		}
	default:
		return fmt.Errorf("unknow message type:%d", msg.MessageType)
	}

	select {
	case c.sendChan <- cs:
	case err := <-c.errChan:
		c.onError(err)
		return err
	}
	return nil
}

func (c *RtmpClient) destroy() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *RtmpClient) mainloop(isPublish bool) {
	defer c.destroy()
	if isPublish {
		// 发送onMateData
		for _, track := range c.tracks {
			video := track.VideoInfo()
			audio := track.AudioInfo()

			objmap := make(amf.Object)
			if audio != nil {
				objmap["audiocodecid"] = audio.CodecID
			}

			if video != nil {
				objmap["videocodecid"] = video.CodecID
				objmap["height"] = video.Height
				objmap["width"] = video.Width
			}

			bw := &bytes.Buffer{}
			if _, err := c.conn.Encode(bw, "onMetaData", amf.AMF0); err != nil {
				c.logger.Errorf("encode value failed, %v", err)
				return
			}
			if _, err := c.conn.Encode(bw, objmap, amf.AMF0); err != nil {
				c.logger.Errorf("encode value failed, %v", err)
				return
			}

			sendBytes := bw.Bytes()
			cs := &core.ChunkStream{
				Data:      sendBytes,
				Length:    uint32(len(sendBytes)),
				StreamID:  track.streamID,
				Timestamp: 0,
				TypeID:    av.TAG_SCRIPTDATAAMF0,
			}

			if err := c.conn.Write(cs); err != nil {
				c.logger.Errorf("send on matedata failed, %v", err)
				return
			}
		}
	}

	for {
		select {
		case cs := <-c.sendChan:
			if err := c.conn.Write(cs); err != nil {
				c.logger.Errorf("write chunk failed, %v", err)
				return
			}
			if err := c.conn.Flush(); err != nil {
				c.logger.Errorf("flush failed, %v", err)
				return
			}
		case err := <-c.errChan:
			c.onError(err)
			return
		case <-c.closed:
			c.onError(errors.New("closed"))
			return
		}
	}
}

func (c *RtmpClient) readloop() {
	defer func() {
		select {
		case err := <-c.errChan:
			c.onConnectState(StateDisconnect, err)
			c.onError(err)
		default:
			c.onConnectState(StateDisconnect, fmt.Errorf("unknow error"))
		}
	}()

	var message *StreamMessage
	for {
		cs, err := c.conn.Read()
		if err != nil {
			c.logger.Errorf("read chunk failed, %v", err)
			c.onError(err)
			return
		}

		if len(cs.Data) == 0 {
			c.logger.Warnf("read 0 size chunk, type:%d", cs.TypeID)
			continue
		}

		switch cs.TypeID {
		case 18, 15: //数据消息,传递一些元数据 amf0-18, amf3-15
			// 脚本要先decode
			var vs []interface{}
			r := bytes.NewReader(cs.Data)
			if cs.TypeID == 18 {
				vs, err = c.conn.DecodeBatch(r, amf.AMF0)
			} else {
				vs, err = c.conn.DecodeBatch(r, amf.AMF3)
			}

			if err != nil && err != io.EOF {
				c.logger.Errorf("decode batch failed, %v", err)
				c.onError(err)
				return
			}

			message = &StreamMessage{
				MessageType: MessageTypeMateData,
				Pts:         cs.Timestamp,
				Dts:         0,
				Payload:     cs.Data,
				extend:      vs,
			}
		case 19, 16: //共享对象消息, afm0-19, afm3-16
			//忽略共享消息？？
			c.logger.Warn("shared message received.")
			continue
		case 8: // 8-音频数据
			header, payload, err := c.demuxer.DemuxAudio(cs.Data)
			if err != nil {
				c.onError(fmt.Errorf("demux audio failed, %v", err))
				return
			}
			message = &StreamMessage{
				MessageType: MessageTypeAudio,
				Pts:         cs.Timestamp,
				Dts:         0,
				Payload:     payload,
				extend:      header,
			}
		case 9: // 9-视频数据
			header, payload, err := c.demuxer.DemuxVideo(cs.Data)
			if err != nil {
				c.onError(fmt.Errorf("demux video failed, %v", err))
				return
			}
			message = &StreamMessage{
				MessageType: MessageTypeVideo,
				Pts:         cs.Timestamp,
				Dts:         0,
				Payload:     payload,
				extend:      header,
			}
		case 22: //组合消息
			//忽略组合消息？？
			c.logger.Warn("aggregage message received.")
			continue
		case 4: //用户控制消息
			//发送connect后，会接收到用户控制消息，比如Stream Begin
			//todo 如何解析用户消息
			c.logger.Warn("user control message received.")
			continue //忽略该消息
		case 20, 17: //控制消息 amf0-20, amf3-17
			// 脚本要先decode
			var vs []interface{}
			r := bytes.NewReader(cs.Data)
			if cs.TypeID == 20 {
				vs, err = c.conn.DecodeBatch(r, amf.AMF0)
			} else {
				vs, err = c.conn.DecodeBatch(r, amf.AMF3)
			}

			if err != nil && err != io.EOF {
				c.onError(fmt.Errorf("decode batch failed, %v", err))
				return
			}

			for _, value := range vs {
				fmt.Println("Debug.... ", value)
			}
			continue
		default:
			c.logger.Warnf("unknow message type:%d", cs.TypeID)
			continue
		}

		inchan, exist := c.streamChans[int(cs.StreamID)]
		if !exist {
			track := newStreamTrack(cs.StreamID, nil, nil, c)
			if c.onStreamTrack != nil {
				c.onStreamTrack(track)
				inchan = make(chan *StreamMessage, 8)
				c.streamChans[int(cs.StreamID)] = inchan
			}
		}

		select {
		case inchan <- message:
		case err := <-c.errChan:
			c.onError(err)
			return
		}
	}
}

func (c *RtmpClient) onError(err error) {
	select {
	case c.errChan <- err:
	default:
	}
}
