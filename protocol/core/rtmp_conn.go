package core

import (
	"encoding/binary"
	"net"
	"time"

	"github.com/fabo871218/srtmp/utils"
)

const (
	_ = iota
	idSetChunkSize
	idAbortMessage
	idAck
	idUserControlMessages
	idWindowAckSize
	idSetPeerBandwidth
)

//RtmpConn ...
type RtmpConn struct {
	net.Conn
	chunkSize           uint32
	remoteChunkSize     uint32
	windowAckSize       uint32
	remoteWindowAckSize uint32
	received            uint32
	ackReceived         uint32
	rw                  *ReadWriter
	pool                *utils.Pool
	chunks              map[uint32]*ChunkStream
}

//NewRtmpConn ...
func NewRtmpConn(c net.Conn, bufferSize int) *RtmpConn {
	return &RtmpConn{
		Conn:                c,
		chunkSize:           128,
		remoteChunkSize:     128,
		windowAckSize:       2500000,
		remoteWindowAckSize: 2500000,
		pool:                utils.NewPool(),
		rw:                  NewReadWriter(c, bufferSize),
		chunks:              make(map[uint32]*ChunkStream),
	}
}

func (rtmpConn *RtmpConn) Read() (c *ChunkStream, err error) {
	var rb byte
	for {
		//读取第一个字节
		if rb, err = rtmpConn.rw.ReadByte(); err != nil {
			return nil, err
		}
		format := uint32(rb >> 6) //获取fmt，前面两位是fmt
		csid := uint32(rb & 0x3f) //获取csid，先获取后面6位的csid
		switch csid {
		case 0: //csid有2个字节，需要再读取一个字节
			if rb, err = rtmpConn.rw.ReadByte(); err != nil {
				return nil, err
			}
			csid = uint32(rb) + 64 //从64开始计算
		case 1:
			if csid, err = rtmpConn.rw.ReadUintLE(2); err != nil {
				return nil, err
			}
			csid += 64 //从64开始计算
		case 2: //表示该chunk是控制信息和命令信息，相当于控制消息的csid就是2
		default: //该6位就是一个csid， 不用处理
		}

		cs, ok := rtmpConn.chunks[csid]
		if !ok { //如果没找到，就创建一个新的chunkstream
			cs = &ChunkStream{
				CSID: csid,
			}
			rtmpConn.chunks[csid] = cs
		}
		cs.tmpFromat = format
		if err = cs.readChunk(rtmpConn.rw, rtmpConn.remoteChunkSize); err != nil {
			return nil, err
		}
		//判断当前chunk是否读取完成
		if cs.isComplete() {
			c = &ChunkStream{
				Format:    cs.Format,
				CSID:      cs.CSID,
				Timestamp: cs.Timestamp,
				Length:    cs.Length,
				TypeID:    cs.TypeID,
				StreamID:  cs.StreamID,
				Data:      cs.Data[0:cs.Length],
			}
			//如果是控制消息，就直接处理掉，不反回到外层
			isHandled := rtmpConn.handleControlMsg(cs)
			rtmpConn.ack(cs.Length)
			if !isHandled {
				return
			}
		}
	}
}

func (rtmpConn *RtmpConn) Write(c *ChunkStream) error {
	if c.TypeID == idSetChunkSize {
		rtmpConn.chunkSize = binary.BigEndian.Uint32(c.Data)
	}
	return c.writeChunk(rtmpConn.rw, int(rtmpConn.chunkSize))
}

//Flush ...
func (rtmpConn *RtmpConn) Flush() error {
	return rtmpConn.rw.Flush()
}

//Close ...
func (rtmpConn *RtmpConn) Close() error {
	return rtmpConn.Conn.Close()
}

//RemoteAddr ...
func (rtmpConn *RtmpConn) RemoteAddr() net.Addr {
	return rtmpConn.Conn.RemoteAddr()
}

//LocalAddr ...
func (rtmpConn *RtmpConn) LocalAddr() net.Addr {
	return rtmpConn.Conn.LocalAddr()
}

//SetDeadline ...
func (rtmpConn *RtmpConn) SetDeadline(t time.Time) error {
	return rtmpConn.Conn.SetDeadline(t)
}

//NewAck ...
func (rtmpConn *RtmpConn) NewAck(size uint32) ChunkStream {
	return initControlMsg(idAck, 4, size)
}

//NewSetChunkSize ...
func (rtmpConn *RtmpConn) NewSetChunkSize(size uint32) ChunkStream {
	return initControlMsg(idSetChunkSize, 4, size)
}

//NewWindowAckSize ...
func (rtmpConn *RtmpConn) NewWindowAckSize(size uint32) ChunkStream {
	return initControlMsg(idWindowAckSize, 4, size)
}

//NewSetPeerBandwidth ...
func (rtmpConn *RtmpConn) NewSetPeerBandwidth(size uint32) ChunkStream {
	ret := initControlMsg(idSetPeerBandwidth, 5, size)
	ret.Data[4] = 2
	return ret
}

//handleControlMsg 处理协议层消息
func (rtmpConn *RtmpConn) handleControlMsg(c *ChunkStream) bool {
	switch c.TypeID {
	case idSetChunkSize:
		rtmpConn.remoteChunkSize = binary.BigEndian.Uint32(c.Data)
	case idAbortMessage:
	case idAck:
	case idUserControlMessages:
	case idWindowAckSize:
		rtmpConn.remoteWindowAckSize = binary.BigEndian.Uint32(c.Data)
	case idSetPeerBandwidth:
	default:
		return false
	}
	return true
}

func (rtmpConn *RtmpConn) ack(size uint32) {
	rtmpConn.received += uint32(size)
	rtmpConn.ackReceived += uint32(size)
	if rtmpConn.received >= 0xf0000000 {
		rtmpConn.received = 0
	}
	if rtmpConn.ackReceived >= rtmpConn.remoteWindowAckSize {
		cs := rtmpConn.NewAck(rtmpConn.ackReceived)
		cs.writeChunk(rtmpConn.rw, int(rtmpConn.chunkSize))
		rtmpConn.ackReceived = 0
	}
}

func initControlMsg(id, size, value uint32) ChunkStream {
	ret := ChunkStream{
		Format:   0,
		CSID:     2,
		TypeID:   id,
		StreamID: 0,
		Length:   size,
		Data:     make([]byte, size),
	}
	utils.PutU32BE(ret.Data[:size], value)
	return ret
}

const (
	streamBegin      uint32 = 0
	streamEOF        uint32 = 1
	streamDry        uint32 = 2
	setBufferLen     uint32 = 3
	streamIsRecorded uint32 = 4
	pingRequest      uint32 = 6
	pingResponse     uint32 = 7
)

/*
   +------------------------------+-------------------------
   |     Event Type ( 2- bytes )  | Event Data
   +------------------------------+-------------------------
   Pay load for the ‘User Control Message’.
*/
func (rtmpConn *RtmpConn) userControlMsg(eventType, buflen uint32) ChunkStream {
	var ret ChunkStream
	buflen += 2
	ret = ChunkStream{
		Format:   0,
		CSID:     2,
		TypeID:   4,
		StreamID: 1,
		Length:   buflen,
		Data:     make([]byte, buflen),
	}
	ret.Data[0] = byte(eventType >> 8 & 0xff)
	ret.Data[1] = byte(eventType & 0xff)
	return ret
}

//SetBegin ...
func (rtmpConn *RtmpConn) SetBegin() {
	ret := rtmpConn.userControlMsg(streamBegin, 4)
	for i := 0; i < 4; i++ {
		ret.Data[2+i] = byte(1 >> uint32((3-i)*8) & 0xff)
	}
	rtmpConn.Write(&ret)
}

//SetRecorded ...
func (rtmpConn *RtmpConn) SetRecorded() {
	ret := rtmpConn.userControlMsg(streamIsRecorded, 4)
	for i := 0; i < 4; i++ {
		ret.Data[2+i] = byte(1 >> uint32((3-i)*8) & 0xff)
	}
	rtmpConn.Write(&ret)
}
