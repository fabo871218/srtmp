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
	chunks              map[uint32]ChunkStream
}

func NewConn(c net.Conn, bufferSize int) *RtmpConn {
	return &RtmpConn{
		Conn:                c,
		chunkSize:           128,
		remoteChunkSize:     128,
		windowAckSize:       2500000,
		remoteWindowAckSize: 2500000,
		pool:                utils.NewPool(),
		rw:                  NewReadWriter(c, bufferSize),
		chunks:              make(map[uint32]ChunkStream),
	}
}

func (rtmpConn *RtmpConn) Read(c *ChunkStream) error {
	for {
		h, _ := rtmpConn.rw.ReadUintBE(1) //读取第一个字节
		format := h >> 6                  //获取fmt
		csid := h & 0x3f                  //获取csid
		cs, ok := rtmpConn.chunks[csid]   //判断csid todo 这里有问题，csid可能会有多个字节
		if !ok {
			//如果没找到，就创建一个新的chunkstream
			cs = ChunkStream{}
			rtmpConn.chunks[csid] = cs
		}
		cs.tmpFromat = format
		cs.CSID = csid
		err := cs.readChunk(rtmpConn.rw, rtmpConn.remoteChunkSize, rtmpConn.pool)
		if err != nil {
			return err
		}
		rtmpConn.chunks[csid] = cs
		//判断当前chunk是否读取完成
		if cs.full() {
			*c = cs
			break
		}
	}

	rtmpConn.handleControlMsg(c)
	rtmpConn.ack(c.Length)
	return nil
}

func (rtmpConn *RtmpConn) Write(c *ChunkStream) error {
	if c.TypeID == idSetChunkSize {
		rtmpConn.chunkSize = binary.BigEndian.Uint32(c.Data)
	}
	return c.writeChunk(rtmpConn.rw, int(rtmpConn.chunkSize))
}

func (rtmpConn *RtmpConn) Flush() error {
	return rtmpConn.rw.Flush()
}

func (rtmpConn *RtmpConn) Close() error {
	return rtmpConn.Conn.Close()
}

func (rtmpConn *RtmpConn) RemoteAddr() net.Addr {
	return rtmpConn.Conn.RemoteAddr()
}

func (rtmpConn *RtmpConn) LocalAddr() net.Addr {
	return rtmpConn.Conn.LocalAddr()
}

func (rtmpConn *RtmpConn) SetDeadline(t time.Time) error {
	return rtmpConn.Conn.SetDeadline(t)
}

func (rtmpConn *RtmpConn) NewAck(size uint32) ChunkStream {
	return initControlMsg(idAck, 4, size)
}

func (rtmpConn *RtmpConn) NewSetChunkSize(size uint32) ChunkStream {
	return initControlMsg(idSetChunkSize, 4, size)
}

func (rtmpConn *RtmpConn) NewWindowAckSize(size uint32) ChunkStream {
	return initControlMsg(idWindowAckSize, 4, size)
}

func (rtmpConn *RtmpConn) NewSetPeerBandwidth(size uint32) ChunkStream {
	ret := initControlMsg(idSetPeerBandwidth, 5, size)
	ret.Data[4] = 2
	return ret
}

func (rtmpConn *RtmpConn) handleControlMsg(c *ChunkStream) {
	if c.TypeID == idSetChunkSize {
		rtmpConn.remoteChunkSize = binary.BigEndian.Uint32(c.Data)
	} else if c.TypeID == idWindowAckSize {
		rtmpConn.remoteWindowAckSize = binary.BigEndian.Uint32(c.Data)
	}
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

func (rtmpConn *RtmpConn) SetBegin() {
	ret := rtmpConn.userControlMsg(streamBegin, 4)
	for i := 0; i < 4; i++ {
		ret.Data[2+i] = byte(1 >> uint32((3-i)*8) & 0xff)
	}
	rtmpConn.Write(&ret)
}

func (rtmpConn *RtmpConn) SetRecorded() {
	ret := rtmpConn.userControlMsg(streamIsRecorded, 4)
	for i := 0; i < 4; i++ {
		ret.Data[2+i] = byte(1 >> uint32((3-i)*8) & 0xff)
	}
	rtmpConn.Write(&ret)
}
