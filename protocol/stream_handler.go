package protocol

import (
	"fmt"
	"sync"

	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol/core"
)

//StreamHandler 管理RtmpStream，每个RtmpStream代表一路流
type StreamHandler struct {
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
func (h *StreamHandler) getOrCreate(streamInfo StreamInfo) *RtmpStream {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	streamKey := fmt.Sprintf("%s_%s", streamInfo.App, streamInfo.Name)
	if stream, ok := h.streams[streamKey]; ok {
		return stream
	}

	stream := NewStream(streamInfo, h, h.logger)
	h.streams[streamKey] = stream
	go stream.streamLoop()
	h.logger.Infof("Create new stream, id:%s app:%s name:%s", stream.streamID,
		streamInfo.App, streamInfo.Name)
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
	streamInfo := StreamInfo{
		App:  app,
		Name: name,
		URL:  url,
	}

	stream := h.getOrCreate(streamInfo)
	if conn.IsPublisher() {
		reader := NewStreamReader(conn, stream.ID(), h.logger)
		if err := stream.AddReader(reader); err != nil {
			return fmt.Errorf("Add stream reader failed, %v", err)
		}
	} else {
		writer := NewStreamWriter(conn, stream.ID(), h.logger)
		if err := stream.AddWriter(writer); err != nil {
			return fmt.Errorf("Add stream writer failed, %v", err)
		}
	}
	return nil
}
