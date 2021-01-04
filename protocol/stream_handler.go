package protocol

import (
	"fmt"
	"sync"

	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol/core"
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
func (h *StreamHandler) getOrCreate(streamInfo StreamInfo1) *RtmpStream {
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

// HandleConnect ...
func (h *StreamHandler) HandleConnect(conn *core.ForwardConnect) error {
	app, name, url := conn.GetStreamInfo()
	streamInfo := StreamInfo1{
		Key:  "", //todo
		App:  app,
		Name: name,
		URL:  url,
	}

	stream := h.getOrCreate(streamInfo)
	if conn.IsPublisher() {
		writer := NewStreamWriter(conn, h.logger)
		if err := stream.AddWriter(writer); err != nil {
			return fmt.Errorf("Add stream writer failed, %v", err)
		}
	} else {
		reader := NewStreamReader(conn, h.logger)
		if err := stream.AddReader(reader); err != nil {
			return fmt.Errorf("Add stream reader failed, %v", err)
		}
	}
	return nil
}
