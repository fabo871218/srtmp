package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol/cache"
	"github.com/fabo871218/srtmp/utils"
)

// StreamInfo ...
type StreamInfo struct {
	App  string
	Name string
	URL  string
}

//RtmpStream rtmp流类型
type RtmpStream struct {
	streamID   string
	isStart    bool
	cache      *cache.Cache
	reader     ReadCloser
	writers    []WriteCloser
	streamInfo StreamInfo

	pktChan       chan *av.Packet
	writerChan    chan WriteCloser
	readerChan    chan ReadCloser
	streamHandler *StreamHandler
	logger        logger.Logger
}

//PackWriterCloser packet写对象结构
type PackWriterCloser struct {
	init bool
	w    WriteCloser
}

//NewWriter 创建新的写对象
func (p *PackWriterCloser) NewWriter() (WriteCloser, error) {
	return p.w, nil
}

//NewStream 创建新的rtmp流
func NewStream(streamInfo StreamInfo, handler *StreamHandler, log logger.Logger) *RtmpStream {
	return &RtmpStream{
		streamID:      utils.NewId(),
		streamInfo:    streamInfo,
		cache:         cache.NewCache(),
		streamHandler: handler,
		writers:       make([]WriteCloser, 0),
		writerChan:    make(chan WriteCloser, 1),
		readerChan:    make(chan ReadCloser, 1),
		pktChan:       make(chan *av.Packet, 16),
		logger:        log,
	}
}

//ID 获取rtmp流id
func (s *RtmpStream) ID() string {
	if s.reader != nil {
		return s.streamID
	}
	return ""
}

//GetReader 获取rtmp流读对象
func (s *RtmpStream) GetReader() ReadCloser {
	return s.reader
}

//AddReader 为rtmp流对象添加一个读对象
func (s *RtmpStream) AddReader(r ReadCloser) error {
	go func() {
		s.readerChan <- r
	}()
	return nil
}

//AddWriter 为rtmp流对象添加一个写对象
func (s *RtmpStream) AddWriter(w WriteCloser) error {
	go func() {
		s.writerChan <- w
	}()
	return nil
}

//开始读取流数据
func (s *RtmpStream) startRead(wg *sync.WaitGroup) {
	s.logger.Infof("Start to read data, id:%s", s.streamID)
	wg.Add(1)
	defer wg.Done()
	for {
		pkt := &av.Packet{}
		if err := s.reader.Read(pkt); err != nil {
			s.logger.Errorf("Read pkt failed, %s", err.Error())
			return
		}
		//先缓存数据包
		s.cache.Write(pkt)
		select {
		case s.pktChan <- pkt:
		default:
		}
	}
}

//转发流数据
func (s *RtmpStream) streamLoop() {
	s.logger.Infof("Start stream loop, %s", s.streamID)
	checkTicker := time.NewTicker(time.Second * 30)
	defer func() {
		streamKey := fmt.Sprintf("%s_%s", s.streamInfo.App, s.streamInfo.Name)
		s.streamHandler.remove(streamKey)
		s.close()
		checkTicker.Stop()
		s.logger.Infof("Rtmp stream[%s] exit.", s.streamID)
	}()

	var wg sync.WaitGroup
	lastWriteRemove := time.Now()
	for {
		select {
		case pkt := <-s.pktChan:
			{
				bRemove := false
				for i, w := range s.writers {
					if err := w.Write(pkt); err != nil {
						s.logger.Infof("Write packet failed, %s close writer.", err.Error())
						w.Close() //todo 是否要传递参数
						s.writers[i] = nil
						bRemove = true
					}
				}

				if bRemove {
					for i := 0; i < len(s.writers); {
						if s.writers[i] == nil {
							s.writers = append(s.writers[:i], s.writers[i+1:]...)
						} else {
							i++
						}
					}
					lastWriteRemove = time.Now()
				}
			}
		case w := <-s.writerChan: // 接收到play消息
			{
				//TODO 这个方法不是很好，先这样，后续再优化
				sw, ok := w.(*StreamWriter)
				if ok == false {
					s.logger.Errorf("can not cast writerclose to streamwriter")
					w.Close()
					return
				}
				if err := s.cache.Send(sw.packetQueue); err != nil {
					s.logger.Errorf("Send cache failed, %s", err.Error())
					w.Close()
					return
				}
				s.writers = append(s.writers, w)
			}
		case r := <-s.readerChan: // 接收到push消息
			{
				if s.reader != nil {
					s.reader.Close()
					wg.Wait() //等待读取数据协程结束

					//清除pktChan中的
				CleanLoop:
					for {
						select {
						case <-s.pktChan:
						default:
							break CleanLoop
						}
					}
					//更新一下基本时间戳，保证每个writer的时间戳都是递增的
					for _, w := range s.writers {
						w.CalcBaseTimestamp()
					}
				}
				s.reader = r
				go s.startRead(&wg)
			}
		case <-checkTicker.C:
			{
				//检查是否有writer，没有则释放
				if len(s.writers) == 0 && time.Now().Sub(lastWriteRemove) >= 30 {
					s.logger.Debug("Stream no play...")
					//return
				}

				//检查是否有reader
				if s.reader == nil || !s.reader.Alive() {
					s.logger.Debugf("Stream reader is nil(%v) or not alive, exit", s.reader == nil)
					return
				}

				//检查每个writer是否超时
				for i := 0; i < len(s.writers); {
					w := s.writers[i]
					if !w.Alive() {
						s.writers = append(s.writers[:i], s.writers[i+1:]...)
						w.Close() //todo 是否要传递关闭原因
						lastWriteRemove = time.Now()
					} else {
						i++
					}
				}
			}
		}
	}
}

func (s *RtmpStream) close() {
	if s.reader != nil {
		s.reader.Close()
		s.logger.Infof("[%s] publish closed.", s.streamID)
	}

	//可能writerChan或readerChan中有未处理的writer和reader
	//读取出来，并关闭
CloseLoop:
	for {
		select {
		case w := <-s.writerChan:
			w.Close()
		case r := <-s.readerChan:
			r.Close()
		default:
			break CloseLoop
		}
	}

	for _, writer := range s.writers {
		writer.Close()
	}
}
