package protocol

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/protocol/core"
	"github.com/fabo871218/srtmp/utils"
)

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

//StreamReadWriteCloser todo comment
type StreamReadWriteCloser interface {
	GetStreamInfo() (string, string, string)
	Close()
	Write(core.ChunkStream) error
	Read(c *core.ChunkStream) error
}

//PeerWriter 是代表rtmp连接的写入对象
type PeerWriter struct {
	av.RWBaser
	UID         string
	closed      bool
	conn        StreamReadWriteCloser
	packetQueue chan *av.Packet
	WriteBWInfo StaticsBW
}

//NewPeerWriter 创建一个新的写入对象
func NewPeerWriter(conn StreamReadWriteCloser) *PeerWriter {
	writer := &PeerWriter{
		UID:         utils.NewId(),
		conn:        conn,
		RWBaser:     av.NewRWBaser(time.Second * time.Duration(*writeTimeout)),
		packetQueue: make(chan *av.Packet, maxQueueNum),
		WriteBWInfo: StaticsBW{0, 0, 0, 0, 0, 0, 0, 0},
	}

	//todo 这个是否有必要先检查一下读写情况
	go writer.Check()
	go func() {
		err := writer.SendPacket()
		if err != nil {
			fmt.Printf("writer.SendPacket failed, %v\n", err)
		}
	}()
	return writer
}

//SaveStatics 保存统计信息
func (pw *PeerWriter) SaveStatics(streamid uint32, length uint64, isVideoFlag bool) {
	nowInMS := int64(time.Now().UnixNano() / 1e6)
	pw.WriteBWInfo.StreamID = streamid
	if isVideoFlag {
		pw.WriteBWInfo.VideoDatainBytes = pw.WriteBWInfo.VideoDatainBytes + length
	} else {
		pw.WriteBWInfo.AudioDatainBytes = pw.WriteBWInfo.AudioDatainBytes + length
	}

	if pw.WriteBWInfo.LastTimestamp == 0 {
		pw.WriteBWInfo.LastTimestamp = nowInMS
	} else if (nowInMS - pw.WriteBWInfo.LastTimestamp) >= SAVE_STATICS_INTERVAL {
		diffTimestamp := (nowInMS - pw.WriteBWInfo.LastTimestamp) / 1000

		pw.WriteBWInfo.VideoSpeedInBytesperMS = (pw.WriteBWInfo.VideoDatainBytes - pw.WriteBWInfo.LastVideoDatainBytes) * 8 / uint64(diffTimestamp) / 1000
		pw.WriteBWInfo.AudioSpeedInBytesperMS = (pw.WriteBWInfo.AudioDatainBytes - pw.WriteBWInfo.LastAudioDatainBytes) * 8 / uint64(diffTimestamp) / 1000

		pw.WriteBWInfo.LastVideoDatainBytes = pw.WriteBWInfo.VideoDatainBytes
		pw.WriteBWInfo.LastAudioDatainBytes = pw.WriteBWInfo.AudioDatainBytes
		pw.WriteBWInfo.LastTimestamp = nowInMS
	}
}

//Check 连接状态检测
func (pw *PeerWriter) Check() {
	var c core.ChunkStream
	for {
		if err := pw.conn.Read(&c); err != nil {
			pw.Close()
			return
		}
	}
}

//DropPacket todo
func (pw *PeerWriter) DropPacket(pktQue chan *av.Packet, streamInfo av.StreamInfo) {
	fmt.Printf("[%v] packet queue max!!!\n", streamInfo)
	for i := 0; i < maxQueueNum-84; i++ {
		tmpPkt, ok := <-pktQue
		// try to don't drop audio
		if ok && tmpPkt.IsAudio {
			if len(pktQue) > maxQueueNum-2 {
				fmt.Println("drop audio pkt")
				<-pktQue
			} else {
				pktQue <- tmpPkt
			}

		}

		if ok && tmpPkt.IsVideo {
			videoPkt, ok := tmpPkt.Header.(av.VideoPacketHeader)
			// dont't drop sps config and dont't drop key frame
			if ok && (videoPkt.IsSeq() || videoPkt.IsKeyFrame()) {
				pktQue <- tmpPkt
			}
			if len(pktQue) > maxQueueNum-10 {
				fmt.Println("drop video pkt")
				<-pktQue
			}
		}
	}
	fmt.Printf("packet queue len: %d\n", len(pktQue))
}

//
func (pw *PeerWriter) Write(p *av.Packet) (err error) {
	err = nil
	if pw.closed {
		err = errors.New("PeerWriter closed")
		return
	}
	defer func() {
		if e := recover(); e != nil {
			errString := fmt.Sprintf("PeerWriter has already been closed:%v", e)
			err = errors.New(errString)
		}
	}()
	if len(pw.packetQueue) >= maxQueueNum-24 {
		pw.DropPacket(pw.packetQueue, pw.StreamInfo())
	} else {
		pw.packetQueue <- p
	}
	return
}

//SendPacket todo comment
func (pw *PeerWriter) SendPacket() error {
	Flush := reflect.ValueOf(pw.conn).MethodByName("Flush")
	var cs core.ChunkStream
	for {
		p, ok := <-pw.packetQueue
		if ok {
			cs.Data = p.Data
			cs.Length = uint32(len(p.Data))
			cs.StreamID = p.StreamID
			cs.Timestamp = p.TimeStamp
			cs.Timestamp += pw.BaseTimeStamp()

			if p.IsVideo {
				cs.TypeID = av.TAG_VIDEO
			} else {
				if p.IsMetadata {
					cs.TypeID = av.TAG_SCRIPTDATAAMF0
				} else {
					cs.TypeID = av.TAG_AUDIO
				}
			}

			pw.SaveStatics(p.StreamID, uint64(cs.Length), p.IsVideo)
			pw.SetPreTime()
			pw.RecTimeStamp(cs.Timestamp, cs.TypeID)
			err := pw.conn.Write(cs)
			if err != nil {
				pw.closed = true
				return err
			}
			Flush.Call(nil)
		} else {
			return errors.New("closed")
		}
	}
}

//StreamInfo todo comment
func (pw *PeerWriter) StreamInfo() (ret av.StreamInfo) {
	ret.UID = pw.UID
	_, _, URL := pw.conn.GetStreamInfo()
	ret.URL = URL
	_url, err := url.Parse(URL)
	if err != nil {
		fmt.Printf("Parse url failed, url:%s err:%v\n", URL, err)
	}
	ret.Key = strings.TrimLeft(_url.Path, "/")
	ret.Inter = true
	return
}

//Close todo comment
func (pw *PeerWriter) Close() {
	if !pw.closed {
		close(pw.packetQueue)
	}
	pw.closed = true
	pw.conn.Close()
}

//PeerReader todo comment
type PeerReader struct {
	av.RWBaser
	UID        string
	demuxer    *flv.Demuxer
	conn       StreamReadWriteCloser
	ReadBWInfo StaticsBW
}

//NewPeerReader 创建一个rtmp连接读对象
func NewPeerReader(conn StreamReadWriteCloser) *PeerReader {
	return &PeerReader{
		UID:        utils.NewId(),
		conn:       conn,
		RWBaser:    av.NewRWBaser(time.Second * time.Duration(*writeTimeout)),
		demuxer:    flv.NewDemuxer(),
		ReadBWInfo: StaticsBW{0, 0, 0, 0, 0, 0, 0, 0},
	}
}

//SaveStatics todo comment
func (pr *PeerReader) SaveStatics(streamid uint32, length uint64, isVideoFlag bool) {
	nowInMS := int64(time.Now().UnixNano() / 1e6)

	pr.ReadBWInfo.StreamID = streamid
	if isVideoFlag {
		pr.ReadBWInfo.VideoDatainBytes = pr.ReadBWInfo.VideoDatainBytes + length
	} else {
		pr.ReadBWInfo.AudioDatainBytes = pr.ReadBWInfo.AudioDatainBytes + length
	}

	if pr.ReadBWInfo.LastTimestamp == 0 {
		pr.ReadBWInfo.LastTimestamp = nowInMS
	} else if (nowInMS - pr.ReadBWInfo.LastTimestamp) >= SAVE_STATICS_INTERVAL {
		diffTimestamp := (nowInMS - pr.ReadBWInfo.LastTimestamp) / 1000

		//glog.Infof("now=%d, last=%d, diff=%d", nowInMS, v.ReadBWInfo.LastTimestamp, diffTimestamp)
		pr.ReadBWInfo.VideoSpeedInBytesperMS = (pr.ReadBWInfo.VideoDatainBytes - pr.ReadBWInfo.LastVideoDatainBytes) * 8 / uint64(diffTimestamp) / 1000
		pr.ReadBWInfo.AudioSpeedInBytesperMS = (pr.ReadBWInfo.AudioDatainBytes - pr.ReadBWInfo.LastAudioDatainBytes) * 8 / uint64(diffTimestamp) / 1000

		pr.ReadBWInfo.LastVideoDatainBytes = pr.ReadBWInfo.VideoDatainBytes
		pr.ReadBWInfo.LastAudioDatainBytes = pr.ReadBWInfo.AudioDatainBytes
		pr.ReadBWInfo.LastTimestamp = nowInMS
	}
}

func (pr *PeerReader) Read(p *av.Packet) (err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("rtmp read packet panic: %v\n", r)
		}
	}()

	pr.SetPreTime()
	var cs core.ChunkStream
	for {
		err = pr.conn.Read(&cs)
		if err != nil {
			return err
		}

		if cs.TypeID == av.TAG_AUDIO ||
			cs.TypeID == av.TAG_VIDEO ||
			cs.TypeID == av.TAG_SCRIPTDATAAMF0 ||
			cs.TypeID == av.TAG_SCRIPTDATAAMF3 {
			break
		}
	}

	p.IsAudio = cs.TypeID == av.TAG_AUDIO
	p.IsVideo = cs.TypeID == av.TAG_VIDEO
	p.IsMetadata = cs.TypeID == av.TAG_SCRIPTDATAAMF0 || cs.TypeID == av.TAG_SCRIPTDATAAMF3
	p.StreamID = cs.StreamID
	p.Data = cs.Data
	p.TimeStamp = cs.Timestamp

	pr.SaveStatics(p.StreamID, uint64(len(p.Data)), p.IsVideo)
	pr.demuxer.DemuxH(p)
	return err
}

//Info 返回信息
func (pr *PeerReader) StreamInfo() (ret av.StreamInfo) {
	ret.UID = pr.UID
	_, _, URL := pr.conn.GetStreamInfo()
	ret.URL = URL
	_url, err := url.Parse(URL)
	if err != nil {
		fmt.Printf("Parse url failed, url:%s err:%v\n", URL, err)
	}
	ret.Key = strings.TrimLeft(_url.Path, "/")
	return
}

//Close 关闭读对象
func (pr *PeerReader) Close() {
	pr.conn.Close()
}
