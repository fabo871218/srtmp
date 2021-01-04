package protocol

import (
	"errors"
	"fmt"
	"time"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol/core"
)

const (
	maxQueueNum         = 1024
	saveStaticsInterval = 5000
)

// ReadCloser ...
type ReadCloser interface {
	Close()
	Alive() bool
	Read(*av.Packet) error
}

// WriteCloser ...
type WriteCloser interface {
	Close()
	Alive() bool
	CalcBaseTimestamp()
	Write(*av.Packet) error
}

//StaticsBW todo comment
type StaticsBW struct {
	StreamID               uint32
	VideoDatainBytes       uint64
	LastVideoDatainBytes   uint64
	VideoSpeedInBytesperMS uint64

	AudioDatainBytes       uint64
	LastAudioDatainBytes   uint64
	AudioSpeedInBytesperMS uint64

	LastTimestamp int64
}

//StreamWriter 是代表rtmp连接的写入对象
type StreamWriter struct {
	av.RWBaser
	streamID     string
	closed       bool
	keyframeNeed bool
	conn         *core.ForwardConnect
	packetQueue  chan *av.Packet
	WriteBWInfo  StaticsBW
	logger       logger.Logger
}

//NewStreamWriter 创建一个新的写入对象
func NewStreamWriter(conn *core.ForwardConnect, streamID string, log logger.Logger) *StreamWriter {
	writer := &StreamWriter{
		streamID:     streamID,
		conn:         conn,
		RWBaser:      av.NewRWBaser(time.Second * 10),
		packetQueue:  make(chan *av.Packet, maxQueueNum),
		WriteBWInfo:  StaticsBW{0, 0, 0, 0, 0, 0, 0, 0},
		logger:       log,
		keyframeNeed: true,
	}

	//todo 这个是否有必要先检查一下读写情况
	go writer.Check()
	go func() {
		err := writer.SendPacket()
		if err != nil {
			writer.logger.Errorf("SendPacket failed, %s", err.Error())
		}
	}()
	return writer
}

//SaveStatics 保存统计信息
func (sw *StreamWriter) SaveStatics(streamid uint32, length uint64, isVideoFlag bool) {
	nowInMS := int64(time.Now().UnixNano() / 1e6)
	sw.WriteBWInfo.StreamID = streamid
	if isVideoFlag {
		sw.WriteBWInfo.VideoDatainBytes = sw.WriteBWInfo.VideoDatainBytes + length
	} else {
		sw.WriteBWInfo.AudioDatainBytes = sw.WriteBWInfo.AudioDatainBytes + length
	}

	if sw.WriteBWInfo.LastTimestamp == 0 {
		sw.WriteBWInfo.LastTimestamp = nowInMS
	} else if (nowInMS - sw.WriteBWInfo.LastTimestamp) >= saveStaticsInterval {
		diffTimestamp := (nowInMS - sw.WriteBWInfo.LastTimestamp) / 1000

		sw.WriteBWInfo.VideoSpeedInBytesperMS = (sw.WriteBWInfo.VideoDatainBytes - sw.WriteBWInfo.LastVideoDatainBytes) * 8 / uint64(diffTimestamp) / 1000
		sw.WriteBWInfo.AudioSpeedInBytesperMS = (sw.WriteBWInfo.AudioDatainBytes - sw.WriteBWInfo.LastAudioDatainBytes) * 8 / uint64(diffTimestamp) / 1000

		sw.WriteBWInfo.LastVideoDatainBytes = sw.WriteBWInfo.VideoDatainBytes
		sw.WriteBWInfo.LastAudioDatainBytes = sw.WriteBWInfo.AudioDatainBytes
		sw.WriteBWInfo.LastTimestamp = nowInMS
	}
}

//Check 连接状态检测
func (sw *StreamWriter) Check() {
	for {
		_, err := sw.conn.Read()
		if err != nil {
			sw.Close()
			return
		}
	}
}

//Write ...
func (sw *StreamWriter) Write(p *av.Packet) (err error) {
	if sw.closed {
		err = errors.New("PeerWriter closed")
		return
	}
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("Panic %v", e)
		}
	}()

	if p.PacketType == av.PacketTypeVideo {
		if sw.keyframeNeed {
			if p.VHeader.FrameType != av.FRAME_KEY {
				sw.logger.Warn("Key frame need.")
				return
			}
			sw.keyframeNeed = false
		}
	}

	select {
	case sw.packetQueue <- p:
	default:
		if p.PacketType == av.PacketTypeVideo && p.VHeader.FrameType == av.FRAME_KEY {
			sw.keyframeNeed = true
		}
		sw.logger.Warn("packet droped...")
	}
	return
}

//SendPacket todo comment
func (sw *StreamWriter) SendPacket() error {
	var cs core.ChunkStream
	for {
		p, ok := <-sw.packetQueue
		if ok {
			cs.Data = p.Data
			cs.Length = uint32(len(p.Data))
			cs.StreamID = p.StreamID
			cs.Timestamp = p.TimeStamp
			cs.Timestamp += sw.BaseTimeStamp()

			isVideo := false
			switch p.PacketType {
			case av.PacketTypeVideo:
				cs.TypeID = av.TAG_VIDEO
				isVideo = true
			case av.PacketTypeAudio:
				cs.TypeID = av.TAG_AUDIO
			case av.PacketTypeMetadata:
				cs.TypeID = av.TAG_SCRIPTDATAAMF0
			}
			sw.SaveStatics(p.StreamID, uint64(cs.Length), isVideo)
			sw.SetPreTime()
			sw.RecTimeStamp(cs.Timestamp, cs.TypeID)
			err := sw.conn.Write(cs)
			if err != nil {
				sw.closed = true
				return err
			}
			sw.conn.Flush()
		} else {
			return errors.New("closed")
		}
	}
}

//StreamInfo todo comment
// func (sw *StreamWriter) StreamInfo() (ret av.StreamInfo) {
// 	ret.UID = sw.UID
// 	_, _, URL := sw.conn.GetStreamInfo()
// 	ret.URL = URL
// 	_url, err := url.Parse(URL)
// 	if err != nil {
// 		fmt.Printf("Parse url failed, url:%s err:%v\n", URL, err)
// 	}
// 	ret.Key = strings.TrimLeft(_url.Path, "/")
// 	ret.Inter = true
// 	return
// }

//Close todo comment
func (sw *StreamWriter) Close() {
	if !sw.closed {
		close(sw.packetQueue)
	}
	sw.closed = true
	sw.conn.Close()
}

//StreamReader todo comment
type StreamReader struct {
	av.RWBaser
	streamID   string
	demuxer    *flv.Demuxer
	conn       *core.ForwardConnect
	ReadBWInfo StaticsBW
	logger     logger.Logger
}

//NewStreamReader 创建一个rtmp连接读对象
func NewStreamReader(conn *core.ForwardConnect, streamID string, log logger.Logger) *StreamReader {
	return &StreamReader{
		streamID:   streamID,
		conn:       conn,
		RWBaser:    av.NewRWBaser(time.Second * 10),
		demuxer:    flv.NewDemuxer(),
		ReadBWInfo: StaticsBW{0, 0, 0, 0, 0, 0, 0, 0},
		logger:     log,
	}
}

//SaveStatics todo comment
func (pr *StreamReader) SaveStatics(streamid uint32, length uint64, isVideoFlag bool) {
	nowInMS := int64(time.Now().UnixNano() / 1e6)

	pr.ReadBWInfo.StreamID = streamid
	if isVideoFlag {
		pr.ReadBWInfo.VideoDatainBytes = pr.ReadBWInfo.VideoDatainBytes + length
	} else {
		pr.ReadBWInfo.AudioDatainBytes = pr.ReadBWInfo.AudioDatainBytes + length
	}

	if pr.ReadBWInfo.LastTimestamp == 0 {
		pr.ReadBWInfo.LastTimestamp = nowInMS
	} else if (nowInMS - pr.ReadBWInfo.LastTimestamp) >= saveStaticsInterval {
		diffTimestamp := (nowInMS - pr.ReadBWInfo.LastTimestamp) / 1000

		//glog.Infof("now=%d, last=%d, diff=%d", nowInMS, v.ReadBWInfo.LastTimestamp, diffTimestamp)
		pr.ReadBWInfo.VideoSpeedInBytesperMS = (pr.ReadBWInfo.VideoDatainBytes - pr.ReadBWInfo.LastVideoDatainBytes) * 8 / uint64(diffTimestamp) / 1000
		pr.ReadBWInfo.AudioSpeedInBytesperMS = (pr.ReadBWInfo.AudioDatainBytes - pr.ReadBWInfo.LastAudioDatainBytes) * 8 / uint64(diffTimestamp) / 1000

		pr.ReadBWInfo.LastVideoDatainBytes = pr.ReadBWInfo.VideoDatainBytes
		pr.ReadBWInfo.LastAudioDatainBytes = pr.ReadBWInfo.AudioDatainBytes
		pr.ReadBWInfo.LastTimestamp = nowInMS
	}
}

func (pr *StreamReader) Read(p *av.Packet) (err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("rtmp read packet panic: %v\n", r)
		}
	}()

	pr.SetPreTime()
	var cs *core.ChunkStream
	for {
		if cs, err = pr.conn.Read(); err != nil {
			return err
		}

		if cs.TypeID == av.TAG_AUDIO ||
			cs.TypeID == av.TAG_VIDEO ||
			cs.TypeID == av.TAG_SCRIPTDATAAMF0 ||
			cs.TypeID == av.TAG_SCRIPTDATAAMF3 {
			break
		}
	}

	isVideo := false
	switch cs.TypeID {
	case av.TAG_VIDEO:
		p.PacketType = av.PacketTypeVideo
		isVideo = true
	case av.TAG_AUDIO:
		p.PacketType = av.PacketTypeAudio
	case av.TAG_SCRIPTDATAAMF0, av.TAG_SCRIPTDATAAMF3:
		p.PacketType = av.PacketTypeMetadata
	}
	p.StreamID = cs.StreamID
	p.Data = cs.Data
	p.TimeStamp = cs.Timestamp

	pr.SaveStatics(p.StreamID, uint64(len(p.Data)), isVideo)
	pr.demuxer.DemuxH(p)
	return err
}

//StreamInfo 返回信息
// func (pr *StreamReader) StreamInfo() (ret av.StreamInfo) {
// 	ret.UID = pr.UID
// 	_, _, URL := pr.conn.GetStreamInfo()
// 	ret.URL = URL
// 	_url, err := url.Parse(URL)
// 	if err != nil {
// 		fmt.Printf("Parse url failed, url:%s err:%v\n", URL, err)
// 	}
// 	ret.Key = strings.TrimLeft(_url.Path, "/")
// 	return
// }

//Close 关闭读对象
func (pr *StreamReader) Close() {
	pr.conn.Close()
}
