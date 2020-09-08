package protocol

import (
	"fmt"
	"sync"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/logger"
)

//StreamHandler 管理RtmpStream，每个RtmpStream代表一路流
type StreamHandler struct {
	//streams cmap.ConcurrentMap //key

	mutex   sync.Mutex
	logger  logger.Logger
	streams map[string]*RtmpStream
}

//NewStreamHandler 创建一个管理RtmpStream的Handler
func NewStreamHandler(log logger.Logger) *StreamHandler {
	handler := &StreamHandler{
		logger:  log,
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

	stream := NewStream(streamInfo, h, h.logger)
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
