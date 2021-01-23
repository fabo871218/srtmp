package core

import (
	"encoding/binary"
	"fmt"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/utils"
)

/*
+--------------+----------------+--------------------+--------------+
| Basic Header | Message Header | Extended Timestamp |  Chunk Data  |
+--------------+----------------+--------------------+--------------+
|                                                    |
|<------------------- Chunk Header ----------------->|
							Chunk Format
Basic Header(1-3字节)，这个字段包含块流ID（csid）和块类型（fmt），fmt取值0-3，定义4中不同的块消息类型
rtmp协议支持用户自定义[3,65599]之间的CSID，0，1，2由协议保留，表示特殊信息。
0-代表basic header总共要占用2个字节，csid在[64,319]之间
1-代表占用3个字节，csid在【64，65599】之间
2-代表chunk是控制信息和一些命令信息
如果第一个字节的csid取值大于2，则说明这个就是csid
     0 1 2 3 4 5 6 7
    +-+-+-+-+-+-+-+-+
    |fmt|   cs id   |
	+-+-+-+-+-+-+-+-+

	 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
    |fmt|    0      |  cs id - 64   |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

     0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
    |fmt|    1      |          cs id - 64           |
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
Message Header(0,3,7,11字节)：这个字段包含被发送的消息信息（无论是全部，还是部分）。字段长度由块头中的块类型（fmt）来决定
类型0--有11个字节组成，其他三种能表示的数据它都能表示，但在chunk stream的开始的第一个chunk和头信息中的时间戳后腿（即值与上
一个chunk相比减小，通常在回退播放的时候会出现这种情况）的时候，必须采用这种格式
     0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
    |                    timestamp                  |message length |
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
    |    message length (coutinue)  |message type id| msg stream id |
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
    |                  msg stream id                |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	timestamp（时间戳）：占用3个字节，因此它最多能表示到16777215=0xFFFFFF=2^24-1，当它
						的值超过这个最大值时，这三个字节都置为1，这样实际的timestamp会转存到 Extended
						Timestamp 字段中，接收端在判断timestamp字段24个位都为1时就会去Extended Timestamp
						中解析实际的时间戳。
	message length（消息数据长度）：占用3个字节，表示实际发送的消息的数据如音频帧、视频
						帧等数据的长度，单位是字节。注意这里是Message的长度，也就是chunk属于的Message的总长
						度，而不是chunk本身data的长度。
	message type id(消息的类型id)：1个字节，表示实际发送的数据的类型，主要分为一下几类
						* 协议控制消息
							SetChunkSize(type id=1):设置chunk中Data字段所能承载的最大字节数，默认是128Bytes，通信过程中可以通过发送该消息来设置chunk size的大小(不小于128B)，
								而且通信的双方各自维护一个chunksize，两端的chunksize是独立的，比如当A想向B发送一个200B的message，但默认的chunksize是128B，因此就要将消息拆分为
								Data分别为128B和72B的两个chunk发送，如果此时先发送一个设置chunksize为256B的消息，再发送Data为200B的chunk，本地不再划分message，B接收到的setchunksize
								的协议控制消息会调整接收的chunk的Data的大小
							Abort Message(type id=2)：当一个Message被切分为多个chunk，接收端只接收到部分chunk是，发送该控制消息表示发送端不在传输痛Message的chunk，接收端接收到这个
								消息后，要丢弃这些不完整的chunk。Data数据中只需要一个CSID，表示丢弃该CSID的所有已接收到的chunk
							Acknowledgement(type id=3): 当接收到对端的消息大小等于窗口大小(window size)时接收端要回馈一个ack给发送端告知对方可以继续发送数据。窗口大小就是指接收到
								接收端返回的ack前最多可以发送的字节数量，返回的ack中会带有从发送上衣额ack后接收到的字节数
							Window Acknowledgement Size(type id=5): 发送端在接收到接收端返回的连个ack间最多可以发送的字节数
							Set Peer Bandwidth(type id=6): 限制对端的输出带宽。接收端接收到该消息后会通过设置消息中的Window ACK Size来限制已发送但未接收到反馈的消息的大小来限制
								发送端的发送带宽。如果消息中的Window Ack Size与上一次发送给发送端的size不通的话要回馈一个Window Acknowledgement Size的控制消息
								Hard(Limit Type=0)：接收端应该将Window Ack Size设置为消息中的值
								Soft(Limit Type=1)：接收端可以将Window Ack Size设置为消息中的值，也可以保存原来的值(前提是原来的size小于该控制消息中的Window Ack Size)
								Dynamic(Limit Type=2)：如果上次的Set Peer BandWidth消息中的Limit Type为0，本次也按Hard处理，否则忽略本消息，不去设置Window Ack Size
						* 数据消息
							8--音频数据  9--视频数据  18--Metadata 包括音视频编码，视频宽高等信息
						* 命令消息
							20，17, 此类消息主要有NetConnection和NetStream两类，两个类分别有多个函数，该
							消息的调用，可理解为远程函数调用
						9代表视频数据。
	message stream id(消息的流id)：4个字节，表示该chunk所在的流的ID，和Basic Header
						的CSID一样，它采用小端存储方式。
类型1-由7个字节组成，省去来表示message stream id的4个字节，表示此chunk和上一次发的chunk所在的流相同，如果在发送端值和对端
	有一个流连接的时候，可以尽量去采用这种格式
     0               1               2               3
     0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
    |               timestamp delta                 |message length |
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
    |    message length (coutinue)  |message type id|
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	timestamp delta：3 bytes，这里和type=0时不同，存储的是和上一个chunk的时间差。类似
					上面提到的timestamp，当它的值超过3个字节所能表示的最大值时，三个字节都置为1，实际
					的时间戳差值就会转存到Extended Timestamp字段中，接收端在判断timestamp delta字段24
					个bit都为1时就会去Extended Timestamp 中解析实际的与上次时间戳的差值。
					其他字段与上面的解释相同.
类型2-type 为 2 时占用 3 个字节，相对于 type = 1 格式又省去了表示消息长度的3个字节和表示消息类型的1个字节，表示此 chunk
		和上一次发送的 chunk 所在的流、消息的长度和消息的类型都相同。余下的这三个字节表示 timestamp delta，使用同type=1。
     0               1               2
     0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
    +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
    |               timestamp delta                 |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

类型3-为0字节，表示这个chunk的Message Header和上一个是完全相同的。当它跟在type=0的chunk后面时，表示和前一
		个 chunk 的时间戳都是相同。什么时候连时间戳都是相同呢？就是一个 Message 拆分成多个 chunk，这个 chunk 和上
		一个 chunk 同属于一个 Message。而当它跟在 type = 1或 type = 2 的chunk后面时的chunk后面时，表示和前一个 chunk
		的时间戳的差是相同的。比如第一个 chunk 的 type = 0，timestamp = 100，第二个 chunk 的 type = 2，
		timestamp delta = 20，表示时间戳为 100 + 20 = 120，第三个 chunk 的 type = 3，表示 timestamp delta = 20,
		时间戳为 120 + 20 = 140。
Extended Timestamp（0，4字节）：这个字段是否存在取决于块消息头中编码的时间戳
	在 chunk 中会有时间戳 timestamp 和时间戳差 timestamp delta，并且它们不会同时存在，只有这两者之一大于3字节能表示的
	最大数值 0xFFFFFF ＝ 16777215 时，才会用这个字段来表示真正的时间戳，否则这个字段为 0。扩展时间戳占 4 个字节，
	能表示的最大数值就是 0xFFFFFFFF ＝ 4294967295。当扩展时间戳启用时，timestamp字段或者timestamp delta要全置为1，
	而不是减去时间戳或者时间戳差的值。
Chunk Data（可变大小）：当前块的有效数据，上限为配置的最大块大小
*/
//ChunkStream 表示一个完整的message
type ChunkStream struct {
	Format    uint32 //2bit 代表chunk message type
	CSID      uint32 //chunk stream id
	Timestamp uint32 //时间戳
	Length    uint32
	TypeID    uint32
	StreamID  uint32
	Data      []byte
	Pts       uint32
	Dts       uint32

	timeDelta uint32 //时间戳扩展
	exted     bool
	index     uint32
	remain    uint32
	complete  bool
	tmpFromat uint32
}

func (cs *ChunkStream) isComplete() bool {
	return cs.complete
}

func (cs *ChunkStream) alloc() {
	cs.complete = false
	cs.index = 0
	cs.remain = cs.Length
	if len(cs.Data) < int(cs.Length) {
		cs.Data = make([]byte, cs.Length)
	}
}

func (cs *ChunkStream) writeHeader(w *ReadWriter) error {
	//Chunk Basic Header
	h := cs.Format << 6
	switch {
	case cs.CSID < 64:
		h |= cs.CSID
		w.WriteUintBE(h, 1)
	case cs.CSID-64 < 256:
		h |= 0
		w.WriteUintBE(h, 1)
		w.WriteUintLE(cs.CSID-64, 1)
	case cs.CSID-64 < 65536:
		h |= 1
		w.WriteUintBE(h, 1)
		w.WriteUintLE(cs.CSID-64, 2)
	}
	//Chunk Message Header
	ts := cs.Timestamp
	if cs.Format == 3 {
		goto END
	}
	if cs.Timestamp > 0xffffff {
		ts = 0xffffff
	}
	w.WriteUintBE(ts, 3)
	if cs.Format == 2 {
		goto END
	}
	if cs.Length > 0xffffff {
		return fmt.Errorf("length=%d", cs.Length)
	}
	w.WriteUintBE(cs.Length, 3)
	w.WriteUintBE(cs.TypeID, 1)
	if cs.Format == 1 {
		goto END
	}
	w.WriteUintLE(cs.StreamID, 4)
END:
	//Extended Timestamp
	if ts >= 0xffffff {
		w.WriteUintBE(cs.Timestamp, 4)
	}
	return w.WriteError()
}

func (cs *ChunkStream) writeChunk(w *ReadWriter, chunkSize int) error {
	if cs.TypeID == av.TAG_AUDIO {
		cs.CSID = 4
	} else if cs.TypeID == av.TAG_VIDEO ||
		cs.TypeID == av.TAG_SCRIPTDATAAMF0 ||
		cs.TypeID == av.TAG_SCRIPTDATAAMF3 {
		cs.CSID = 6
	}

	totalLen := uint32(0)
	numChunks := (cs.Length / uint32(chunkSize))
	for i := uint32(0); i <= numChunks; i++ {
		if totalLen == cs.Length {
			break
		}
		if i == 0 {
			cs.Format = uint32(0)
		} else {
			cs.Format = uint32(3)
		}
		if err := cs.writeHeader(w); err != nil {
			return err
		}
		inc := uint32(chunkSize)
		start := uint32(i) * uint32(chunkSize)
		if uint32(len(cs.Data))-start <= inc {
			inc = uint32(len(cs.Data)) - start
		}
		totalLen += inc
		end := start + inc
		buf := cs.Data[start:end]
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func (cs *ChunkStream) readChunk(r *ReadWriter, chunkSize uint32) error {
	var rmark bool = false //表示是不是读取一个message的第一个chunk
	if cs.remain == 0 {
		//如果一个ChunkStream没有剩余的内容没有读取，那么就应该人为上一个message已经结束
		//设置rmark为true
		rmark = true
	}

	var messageHeader [11]byte //message hader最长11个字节长度
	var timeExtend bool = false
	switch cs.tmpFromat {
	case 0: //全类型，一般是一个chunk stream的开始,11个字节长度
		if _, err := r.Read(messageHeader[0:]); err != nil {
			return fmt.Errorf("read message header failed, %v", err)
		}
		cs.Format = cs.tmpFromat
		cs.Timestamp = utils.U24BE(messageHeader[0:]) //timestamp 3个字节
		cs.Length = utils.U24BE(messageHeader[3:])    //3字节
		cs.TypeID = uint32(messageHeader[6])          //一个字节长度
		cs.StreamID = utils.U32LE(messageHeader[7:])  //4个字节
		if cs.Timestamp == 0xFFFFFF {
			timeExtend = true
		}
	case 1: //与上一个属于同一个流, 7个字节长度
		if _, err := r.Read(messageHeader[0:7]); err != nil {
			return fmt.Errorf("read message header failed, %v", err)
		}
		cs.Format = cs.tmpFromat
		cs.timeDelta = utils.U24BE(messageHeader[0:]) //timeDelta 3个字节
		cs.Length = utils.U24BE(messageHeader[3:])    //3字节
		cs.TypeID = uint32(messageHeader[6])          //一个字节长度
		if cs.timeDelta == 0xFFFFFF {
			timeExtend = true
		}
	case 2: //3个字节长度
		if _, err := r.Read(messageHeader[0:3]); err != nil {
			return fmt.Errorf("read message header failed, %v", err)
		}
		cs.Format = cs.tmpFromat
		cs.timeDelta = utils.U24BE(messageHeader[0:]) //timeDelta 3个字节
		if cs.timeDelta == 0xFFFFFF {
			timeExtend = true
		}
	case 3: //0个字节长度
		if cs.timeDelta == 0xFFFFFF {
			timeExtend = true
		}
	default:
		return fmt.Errorf("invalid fmt type:%d", cs.tmpFromat)
	}
	//如果有扩展时间戳，读取扩展时间戳
	if timeExtend {
		if _, err := r.Read(messageHeader[0:4]); err != nil {
			return fmt.Errorf("read time extend failed, %v", err)
		}
	}

	//如果是第一个chunk，设置pts的值，分配空间
	if rmark {
		cs.alloc()
		switch cs.tmpFromat {
		case 0:
			if timeExtend {
				cs.Pts = utils.U32BE(messageHeader[0:])
			} else {
				cs.Pts = cs.Timestamp
			}
		case 1, 2, 3:
			if timeExtend {
				cs.Pts += utils.U32BE(messageHeader[0:])
			} else {
				cs.Pts += cs.timeDelta
			}
		}
	}

	size := int(cs.remain)
	if size > int(chunkSize) {
		size = int(chunkSize)
	}

	dataBuf := cs.Data[cs.index : cs.index+uint32(size)]
	//读取数据
	if _, err := r.Read(dataBuf); err != nil {
		return fmt.Errorf("read chunk data failed, %v", err)
	}
	cs.index += uint32(size)
	cs.remain -= uint32(size)
	if cs.remain == 0 {
		cs.complete = true
	}
	return nil
}

func (cs *ChunkStream) readChunk1(r *ReadWriter, chunkSize uint32, pool *utils.Pool) error {
	if cs.remain != 0 && cs.tmpFromat != 3 {
		//如果remain != 0，说明还有消息没有读取完，所有fmt为类型3？
		return fmt.Errorf("inlaid remin = %d", cs.remain)
	}

	switch cs.tmpFromat {
	case 0: //全类型，一般是一个chunk stream的开始
		cs.Format = cs.tmpFromat
		cs.Timestamp, _ = r.ReadUintBE(3)
		cs.Length, _ = r.ReadUintBE(3)
		cs.TypeID, _ = r.ReadUintBE(1)
		cs.StreamID, _ = r.ReadUintLE(4)
		if cs.Timestamp == 0xffffff {
			cs.Timestamp, _ = r.ReadUintBE(4)
			cs.exted = true
		} else {
			cs.exted = false
		}
		cs.alloc()
	case 1: //与上一个属于同一个流
		cs.Format = cs.tmpFromat
		timeStamp, _ := r.ReadUintBE(3)
		cs.Length, _ = r.ReadUintBE(3)
		cs.TypeID, _ = r.ReadUintBE(1)
		if timeStamp == 0xffffff {
			timeStamp, _ = r.ReadUintBE(4)
			cs.exted = true
		} else {
			cs.exted = false
		}
		cs.timeDelta = timeStamp
		cs.Timestamp += timeStamp
		cs.alloc()
	case 2: //时间戳不一样
		cs.Format = cs.tmpFromat
		timeStamp, _ := r.ReadUintBE(3)
		if timeStamp == 0xffffff {
			timeStamp, _ = r.ReadUintBE(4)
			cs.exted = true
		} else {
			cs.exted = false
		}
		cs.timeDelta = timeStamp
		cs.Timestamp += timeStamp
		cs.alloc()
	case 3: //都一样
		if cs.remain == 0 {
			//如果cs.remain == 0，表示是该message的第一个包，要处理时间戳，
			//所有的同一个message的chunk，时间戳应该是一样的，只处理第一个就可以了
			switch cs.Format {
			case 0:
				if cs.exted {
					timestamp, _ := r.ReadUintBE(4)
					cs.Timestamp = timestamp
				}
			case 1, 2:
				var timedet uint32
				if cs.exted {
					timedet, _ = r.ReadUintBE(4)
				} else {
					timedet = cs.timeDelta
				}
				cs.Timestamp += timedet
			}
			cs.alloc()
		} else {
			if cs.exted {
				b, err := r.Peek(4)
				if err != nil {
					return err
				}
				tmpts := binary.BigEndian.Uint32(b)
				if tmpts == cs.Timestamp {
					r.Discard(4)
				}
			}
		}
	default:
		return fmt.Errorf("invalid format=%d", cs.Format)
	}

	size := int(cs.remain)
	if size > int(chunkSize) {
		size = int(chunkSize)
	}

	buf := cs.Data[cs.index : cs.index+uint32(size)]
	//读取数据
	if _, err := r.Read(buf); err != nil {
		return err
	}
	cs.index += uint32(size)
	cs.remain -= uint32(size)
	if cs.remain == 0 {
		cs.complete = true
	}
	return r.readError
}
