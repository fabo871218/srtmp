package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/protocol/cache"
)

var (
	EmptyID = ""
)

type StreamHandler struct {
	//streams cmap.ConcurrentMap //key

	mutex   sync.Mutex
	streams map[string]*RtmpStream
}

func NewRtmpStream() *StreamHandler {
	handler := &StreamHandler{
		streams: make(map[string]*RtmpStream),
	}
	return handler
}

//get rtmp stream, if not exist, create a new one
//bool indicate weathe the stream is new, true-new false-not
func (h *StreamHandler) getOrCreate(streamInfo av.StreamInfo) *RtmpStream {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if stream, ok := h.streams[streamInfo.Key]; ok {
		return stream
	}

	stream := NewStream(streamInfo, h)
	h.streams[streamInfo.Key] = stream
	go stream.streamLoop()
	return stream
}

func (h *StreamHandler) remove(key string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if _, ok := h.streams[key]; ok {
		delete(h.streams, key)
	}
}

//GetStreams 获取所有的流
func (h *StreamHandler) GetStreams() []*RtmpStream {
	streams := make([]*RtmpStream, 0)
	h.mutex.Lock()
	defer h.mutex.Unlock()
	for _, v := range h.streams {
		streams = append(streams, v)
	}
	return streams
}

//HandleReader 创建和添加一个新的rtmp stream
//todo 要判断是否有错误
func (h *StreamHandler) HandleReader(r av.ReadCloser) error {
	stream := h.getOrCreate(r.StreamInfo())
	if err := stream.AddReader(r); err != nil {
		return fmt.Errorf("stream.AddReader faile, %v", err)
	}
	return nil
	// var stream *RtmpStream
	// i, ok := rs.streams.Get(streamInfo.Key)
	// if stream, ok = i.(*RtmpStream); ok {
	// 	glog.Infof("find stream:%s stop and rebuild.", streamInfo.Key)
	// 	stream.TransStop()
	// 	id := stream.ID()
	// 	if id != EmptyID && id != streamInfo.UID {
	// 		ns := NewStream()
	// 		stream.Copy(ns)
	// 		stream = ns
	// 		rs.streams.Set(streamInfo.Key, ns)
	// 	}
	// } else {
	// 	stream = NewStream()
	// 	rs.streams.Set(streamInfo.Key, stream)
	// 	stream.streamInfo = streamInfo
	// }

	// stream.AddReader(r)
}

//HandleWriter 处理rtmp写入对象
//todo 要判断是否有错误
func (h *StreamHandler) HandleWriter(w av.WriteCloser) error {
	stream := h.getOrCreate(w.StreamInfo())
	if err := stream.AddWriter(w); err != nil {
		return fmt.Errorf("stream.AddWriter faile, %v", err)
	}
	return nil
}

//RtmpStream rtmp流类型
type RtmpStream struct {
	isStart    bool
	cache      *cache.Cache
	reader     av.ReadCloser
	writers    []av.WriteCloser
	streamInfo av.StreamInfo

	pktChan       chan *av.Packet
	writerChan    chan av.WriteCloser
	readerChan    chan av.ReadCloser
	streamHandler *StreamHandler
}

//PackWriterCloser packet写对象结构
type PackWriterCloser struct {
	init bool
	w    av.WriteCloser
}

//NewWriter 创建新的写对象
func (p *PackWriterCloser) NewWriter() (av.WriteCloser, error) {
	return p.w, nil
}

//NewStream 创建新的rtmp流
func NewStream(streamInfo av.StreamInfo, handler *StreamHandler) *RtmpStream {
	return &RtmpStream{
		streamInfo:    streamInfo,
		cache:         cache.NewCache(),
		streamHandler: handler,
		writers:       make([]av.WriteCloser, 0),
		writerChan:    make(chan av.WriteCloser, 1),
		readerChan:    make(chan av.ReadCloser, 1),
		pktChan:       make(chan *av.Packet, 16),
	}
}

//ID 获取rtmp流id
func (s *RtmpStream) ID() string {
	if s.reader != nil {
		return s.reader.StreamInfo().UID
	}
	return EmptyID
}

//GetReader 获取rtmp流读对象
func (s *RtmpStream) GetReader() av.ReadCloser {
	return s.reader
}

//AddReader 为rtmp流对象添加一个读对象
func (s *RtmpStream) AddReader(r av.ReadCloser) error {
	go func() {
		s.readerChan <- r
	}()
	return nil
}

//AddWriter 为rtmp流对象添加一个写对象
func (s *RtmpStream) AddWriter(w av.WriteCloser) error {
	go func() {
		s.writerChan <- w
	}()
	return nil
}

//开始读取流数据
func (s *RtmpStream) startRead(wg *sync.WaitGroup) {
	fmt.Printf("Start to read data, name:%s\n", s.streamInfo.Key)
	wg.Add(1)
	defer wg.Done()
	for {
		pkt := &av.Packet{}
		if err := s.reader.Read(pkt); err != nil {
			fmt.Printf("Read pkt failed, %v\n", err)
			return
		}

		s.cache.Write(*pkt)
		select {
		case s.pktChan <- pkt:
			{
			}
		default:
			{
			}
		}
	}
}

//转发流数据
func (s *RtmpStream) streamLoop() {
	fmt.Printf("start stream loop, %s\n", s.streamInfo.Key)
	checkTicker := time.NewTicker(time.Second * 30)
	defer func() {
		s.streamHandler.remove(s.streamInfo.Key)
		s.close()
		checkTicker.Stop()
		fmt.Printf("rtmp stream[%s] exit.\n", s.streamInfo.Key)
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
						fmt.Printf("write packet failed, %v close writer", err)
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
		case w := <-s.writerChan:
			{
				if err := s.cache.Send(w); err != nil {
					fmt.Printf("s.cache.Send failed, %v\n", err)
					w.Close()
				} else {
					s.writers = append(s.writers, w)
				}
			}
		case r := <-s.readerChan:
			{
				if s.reader != nil {
					s.reader.Close()
					wg.Wait() //等待读取数据协程结束

					//清除pktChan中的
				CleanLoop:
					for {
						select {
						case <-s.pktChan:
							{
							}
						default:
							{
								break CleanLoop
							}
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
					fmt.Println("stream no play...")
					//return
				}

				//检查是否有reader
				if s.reader == nil || !s.reader.Alive() {
					fmt.Printf("stream reader is nil(%v) or not alive, exit\n", s.reader == nil)
					return
				}

				//检查每个writer是否超时
				for i, w := range s.writers {
					if !w.Alive() {
						s.writers = append(s.writers[:i], s.writers[i+1:]...)
						w.Close() //todo 是否要传递关闭原因
						lastWriteRemove = time.Now()
					}
				}
			}
		}
	}
}

func (s *RtmpStream) close() {
	if s.reader != nil {
		s.reader.Close()
		fmt.Printf("[%s] publisher closed\n", s.reader.StreamInfo().Key)
	}

	//可能writerChan或readerChan中有未处理的writer和reader
	//读取出来，并关闭
CloseLoop:
	for {
		select {
		case w := <-s.writerChan:
			{
				w.Close()
			}
		case r := <-s.readerChan:
			{
				r.Close()
			}
		default:
			{
				break CloseLoop
			}
		}
	}

	for _, writer := range s.writers {
		writer.Close()
	}
}
