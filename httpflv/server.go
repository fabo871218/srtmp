// package httpflv

// import (
// 	"encoding/json"
// 	"fmt"
// 	"net"
// 	"net/http"
// 	"strings"

// 	"github.com/fabo871218/srtmp/av"
// )

// type Server struct {
// 	handler av.Handler
// }

// type stream struct {
// 	Key string `json:"key"`
// 	Id  string `json:"id"`
// }

// type streams struct {
// 	Publishers []stream `json:"publishers"`
// 	Players    []stream `json:"players"`
// }

// func NewServer(h av.Handler) *Server {
// 	return &Server{
// 		handler: h,
// 	}
// }

// func (server *Server) Serve(l net.Listener) error {
// 	mux := http.NewServeMux()
// 	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
// 		server.handleConn(w, r)
// 	})
// 	mux.HandleFunc("/streams", func(w http.ResponseWriter, r *http.Request) {
// 		server.getStream(w, r)
// 	})
// 	http.Serve(l, mux)
// 	return nil
// }

// // 获取发布和播放器的信息
// func (server *Server) getStreams(w http.ResponseWriter, r *http.Request) *streams {
// 	// streamHandler := server.handler.(*protocol.StreamHandler)
// 	// if streamHandler == nil {
// 	// 	return nil
// 	// }
// 	// msgs := new(streams)
// 	// for item := range streamHandler.GetStreams().IterBuffered() {
// 	// 	if s, ok := item.Val.(*protocol.RtmpStream); ok {
// 	// 		if s.GetReader() != nil {
// 	// 			msg := stream{item.Key, s.GetReader().StreamInfo().UID}
// 	// 			msgs.Publishers = append(msgs.Publishers, msg)
// 	// 		}
// 	// 	}
// 	// }

// 	// for item := range streamHandler.GetStreams().IterBuffered() {
// 	// 	ws := item.Val.(*protocol.RtmpStream).GetWs()
// 	// 	for s := range ws.IterBuffered() {
// 	// 		if pw, ok := s.Val.(*protocol.PackWriterCloser); ok {
// 	// 			if writer, err := pw.NewWriter(); err == nil && writer != nil {
// 	// 				msg := stream{item.Key, writer.StreamInfo().UID}
// 	// 				msgs.Players = append(msgs.Players, msg)
// 	// 			}
// 	// 		}
// 	// 	}
// 	// }

// 	// return msgs
// 	return nil
// }

// func (server *Server) getStream(w http.ResponseWriter, r *http.Request) {
// 	msgs := server.getStreams(w, r)
// 	if msgs == nil {
// 		return
// 	}
// 	resp, _ := json.Marshal(msgs)
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Write(resp)
// }

// func (server *Server) handleConn(w http.ResponseWriter, r *http.Request) {
// 	defer func() {
// 		if r := recover(); r != nil {
// 			fmt.Printf("http flv handleConn panic: %v\n", r)
// 		}
// 	}()

// 	url := r.URL.String()
// 	u := r.URL.Path
// 	if pos := strings.LastIndex(u, "."); pos < 0 || u[pos:] != ".flv" {
// 		http.Error(w, "invalid path", http.StatusBadRequest)
// 		return
// 	}
// 	path := strings.TrimSuffix(strings.TrimLeft(u, "/"), ".flv")
// 	paths := strings.SplitN(path, "/", 2)
// 	fmt.Printf("url:%s path:%s paths:%s\n", u, path, paths)

// 	if len(paths) != 2 {
// 		http.Error(w, "invalid path", http.StatusBadRequest)
// 		return
// 	}

// 	// 判断视屏流是否发布,如果没有发布,直接返回404
// 	msgs := server.getStreams(w, r)
// 	if msgs == nil || len(msgs.Publishers) == 0 {
// 		http.Error(w, "invalid path", http.StatusNotFound)
// 		return
// 	} else {
// 		include := false
// 		for _, item := range msgs.Publishers {
// 			if item.Key == path {
// 				include = true
// 				break
// 			}
// 		}
// 		if include == false {
// 			http.Error(w, "invalid path", http.StatusNotFound)
// 			return
// 		}
// 	}

// 	w.Header().Set("Access-Control-Allow-Origin", "*")
// 	writer := NewFLVWriter(paths[0], paths[1], url, w)

// 	server.handler.HandleWriter(writer)
// 	writer.Wait()
// }
