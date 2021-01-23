// package httpopera

// import (
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net"
// 	"net/http"

// 	"github.com/fabo871218/srtmp/av"
// 	"github.com/fabo871218/srtmp/protocol"
// )

// type Response struct {
// 	w       http.ResponseWriter
// 	Status  int    `json:"status"`
// 	Message string `json:"message"`
// }

// func (r *Response) SendJson() (int, error) {
// 	resp, _ := json.Marshal(r)
// 	r.w.Header().Set("Content-Type", "application/json")
// 	return r.w.Write(resp)
// }

// type Operation struct {
// 	Method string `json:"method"`
// 	URL    string `json:"url"`
// 	Stop   bool   `json:"stop"`
// }

// type OperationChange struct {
// 	Method    string `json:"method"`
// 	SourceURL string `json:"source_url"`
// 	TargetURL string `json:"target_url"`
// 	Stop      bool   `json:"stop"`
// }

// type ClientInfo struct {
// 	url              string
// 	rtmpRemoteClient *protocol.Client
// 	rtmpLocalClient  *protocol.Client
// }

// type Server struct {
// 	handler  av.Handler
// 	session  map[string]*protocol.RtmpRelay
// 	rtmpAddr string
// }

// func NewServer(h av.Handler, rtmpAddr string) *Server {
// 	return &Server{
// 		handler:  h,
// 		session:  make(map[string]*protocol.RtmpRelay),
// 		rtmpAddr: rtmpAddr,
// 	}
// }

// func (s *Server) Serve(l net.Listener) error {
// 	mux := http.NewServeMux()

// 	mux.Handle("/statics", http.FileServer(http.Dir("statics")))

// 	mux.HandleFunc("/control/push", func(w http.ResponseWriter, r *http.Request) {
// 		s.handlePush(w, r)
// 	})
// 	mux.HandleFunc("/control/pull", func(w http.ResponseWriter, r *http.Request) {
// 		s.handlePull(w, r)
// 	})
// 	mux.HandleFunc("/stat/livestat", func(w http.ResponseWriter, r *http.Request) {
// 		s.GetLiveStatics(w, r)
// 	})
// 	http.Serve(l, mux)
// 	return nil
// }

// type stream struct {
// 	Key             string `json:"key"`
// 	Url             string `json:"Url"`
// 	StreamId        uint32 `json:"StreamId"`
// 	VideoTotalBytes uint64 `json:123456`
// 	VideoSpeed      uint64 `json:123456`
// 	AudioTotalBytes uint64 `json:123456`
// 	AudioSpeed      uint64 `json:123456`
// }

// type streams struct {
// 	Publishers []stream `json:"publishers"`
// 	Players    []stream `json:"players"`
// }

// //http://127.0.0.1:8090/stat/livestat
// func (server *Server) GetLiveStatics(w http.ResponseWriter, req *http.Request) {
// 	streamHandler := server.handler.(*protocol.StreamHandler)
// 	if streamHandler == nil {
// 		io.WriteString(w, "<h1>Get rtmp stream information error</h1>")
// 		return
// 	}

// 	msgs := new(streams)
// 	for _, s := range streamHandler.GetStreams() {
// 		if s.GetReader() != nil {
// 			switch s.GetReader().(type) {
// 			case *protocol.StreamReader:
// 				v := s.GetReader().(*protocol.StreamReader)
// 				msg := stream{v.StreamInfo().Key, v.StreamInfo().URL, v.ReadBWInfo.StreamID, v.ReadBWInfo.VideoSpeedInBytesperMS,
// 					v.ReadBWInfo.AudioDatainBytes, v.ReadBWInfo.AudioSpeedInBytesperMS}
// 				msgs.Publishers = append(msgs.Publishers, msg)
// 			}
// 		}
// 	}

// 	// for item := range streamHandler.GetStreams().IterBuffered() {
// 	// 	ws := item.Val.(*protocol.RtmpStream).GetWs()
// 	// 	for s := range ws.IterBuffered() {
// 	// 		if pw, ok := s.Val.(*protocol.PackWriterCloser); ok {
// 	// 			if writer, err := pw.NewWriter(); err == nil && writer != nil {
// 	// 				switch writer.(type) {
// 	// 				case *protocol.PeerWriter:
// 	// 					v := writer.(*protocol.PeerWriter)
// 	// 					msg := stream{item.Key, v.StreamInfo().URL, v.WriteBWInfo.StreamID, v.WriteBWInfo.VideoDatainBytes, v.WriteBWInfo.VideoSpeedInBytesperMS,
// 	// 						v.WriteBWInfo.AudioDatainBytes, v.WriteBWInfo.AudioSpeedInBytesperMS}
// 	// 					msgs.Players = append(msgs.Players, msg)
// 	// 				}
// 	// 			}
// 	// 		}
// 	// 	}
// 	// }
// 	resp, _ := json.Marshal(msgs)
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Write(resp)
// }

// //http://127.0.0.1:8090/control/push?&oper=start&app=live&name=123456&url=rtmp://192.168.16.136/live/123456
// func (s *Server) handlePull(w http.ResponseWriter, req *http.Request) {
// 	var retString string
// 	var err error

// 	req.ParseForm()

// 	oper := req.Form["oper"]
// 	app := req.Form["app"]
// 	name := req.Form["name"]
// 	url := req.Form["url"]

// 	fmt.Printf("control pull: oper=%v, app=%v, name=%v, url=%v\n", oper, app, name, url)
// 	if (len(app) <= 0) || (len(name) <= 0) || (len(url) <= 0) {
// 		io.WriteString(w, "control push parameter error, please check them.</br>")
// 		return
// 	}

// 	remoteurl := "rtmp://127.0.0.1" + s.rtmpAddr + "/" + app[0] + "/" + name[0]
// 	localurl := url[0]

// 	keyString := "pull:" + app[0] + "/" + name[0]
// 	if oper[0] == "stop" {
// 		pullRtmprelay, found := s.session[keyString]

// 		if !found {
// 			retString = fmt.Sprintf("session key[%s] not exist, please check it again.", keyString)
// 			io.WriteString(w, retString)
// 			return
// 		}
// 		fmt.Printf("rtmprelay stop push %s from %s\n", remoteurl, localurl)
// 		pullRtmprelay.Stop()

// 		delete(s.session, keyString)
// 		retString = fmt.Sprintf("<h1>push url stop %s ok</h1></br>", url[0])
// 		io.WriteString(w, retString)
// 		fmt.Printf("pull stop return %s\n", retString)
// 	} else {
// 		pullRtmprelay := protocol.NewRtmpRelay(&localurl, &remoteurl)
// 		fmt.Printf("rtmprelay start push %s from %s\n", remoteurl, localurl)
// 		err = pullRtmprelay.Start()
// 		if err != nil {
// 			retString = fmt.Sprintf("push error=%v", err)
// 		} else {
// 			s.session[keyString] = pullRtmprelay
// 			retString = fmt.Sprintf("<h1>push url start %s ok</h1></br>", url[0])
// 		}
// 		io.WriteString(w, retString)
// 		fmt.Printf("pull start return %s\n", retString)
// 	}
// }

// //http://127.0.0.1:8090/control/push?&oper=start&app=live&name=123456&url=rtmp://192.168.16.136/live/123456
// func (s *Server) handlePush(w http.ResponseWriter, req *http.Request) {
// 	var retString string
// 	var err error

// 	req.ParseForm()

// 	oper := req.Form["oper"]
// 	app := req.Form["app"]
// 	name := req.Form["name"]
// 	url := req.Form["url"]

// 	fmt.Printf("control push: oper=%v, app=%v, name=%v, url=%v\n", oper, app, name, url)
// 	if (len(app) <= 0) || (len(name) <= 0) || (len(url) <= 0) {
// 		io.WriteString(w, "control push parameter error, please check them.</br>")
// 		return
// 	}

// 	localurl := "rtmp://127.0.0.1" + s.rtmpAddr + "/" + app[0] + "/" + name[0]
// 	remoteurl := url[0]

// 	keyString := "push:" + app[0] + "/" + name[0]
// 	if oper[0] == "stop" {
// 		pushRtmprelay, found := s.session[keyString]
// 		if !found {
// 			retString = fmt.Sprintf("<h1>session key[%s] not exist, please check it again.</h1>", keyString)
// 			io.WriteString(w, retString)
// 			return
// 		}
// 		fmt.Printf("rtmprelay stop push %s from %s\n", remoteurl, localurl)
// 		pushRtmprelay.Stop()

// 		delete(s.session, keyString)
// 		retString = fmt.Sprintf("<h1>push url stop %s ok</h1></br>", url[0])
// 		io.WriteString(w, retString)
// 		fmt.Printf("push stop return %s\n", retString)
// 	} else {
// 		pushRtmprelay := protocol.NewRtmpRelay(&localurl, &remoteurl)
// 		fmt.Printf("rtmprelay start push %s from %s\n", remoteurl, localurl)
// 		err = pushRtmprelay.Start()
// 		if err != nil {
// 			retString = fmt.Sprintf("push error=%v", err)
// 		} else {
// 			retString = fmt.Sprintf("<h1>push url start %s ok</h1></br>", url[0])
// 			s.session[keyString] = pushRtmprelay
// 		}

// 		io.WriteString(w, retString)
// 		fmt.Printf("push start return %s\n", retString)
// 	}
// }
