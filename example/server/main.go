package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/fabo871218/srtmp"
)

// import (
// 	"flag"
// 	"log"
// 	"net"
// 	"time"

// 	"registry.code.tuya-inc.top/TuyaBEMiddleWare/glog"
// 	"github.com/fabo871218/srtmp/hls"
// 	"github.com/fabo871218/srtmp/httpflv"
// 	"github.com/fabo871218/srtmp/httpopera"
// 	"github.com/fabo871218/srtmp/protocol"
// )

// var (
// 	version        = "master"
// 	rtmpAddr       = flag.String("rtmp-addr", ":1935", "RTMP server listen address")
// 	httpFlvAddr    = flag.String("httpflv-addr", ":7001", "HTTP-FLV server listen address")
// 	hlsAddr        = flag.String("hls-addr", ":7002", "HLS server listen address")
// 	operaAddr      = flag.String("manage-addr", ":8090", "HTTP manage interface server listen address")
// 	configfilename = flag.String("cfgfile", ".livego.json", "configure filename")
// )

// func init() {
// 	log.SetFlags(log.Lshortfile | log.Ltime | log.Ldate)
// 	flag.Parse()
// }

// func startHls() *hls.Server {
// 	hlsListen, err := net.Listen("tcp", *hlsAddr)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	hlsServer := hls.NewServer()
// 	go func() {
// 		defer func() {
// 			if r := recover(); r != nil {
// 				glog.Infof("HLS server panic: %v", r)
// 			}
// 		}()
// 		glog.Infof("HLS listen On:%s", *hlsAddr)
// 		hlsServer.Serve(hlsListen)
// 	}()
// 	return hlsServer
// }

// func startRtmp(stream *protocol.StreamHandler, hlsServer *hls.Server) {
// 	var rtmpServer *protocol.Server

// 	if hlsServer == nil {
// 		rtmpServer = protocol.NewRtmpServer(stream, nil)
// 		glog.Info("hls server disable....")
// 	} else {
// 		rtmpServer = protocol.NewRtmpServer(stream, hlsServer)
// 		glog.Info("hls server enable....")
// 	}

// 	defer func() {
// 		if r := recover(); r != nil {
// 			glog.Error("Rtmp server panic:", r)
// 		}
// 	}()
// 	//todo 修改地址
// 	glog.Infof("Start rtmp server, linsten on:1935...")
// 	if err := rtmpServer.Serve(":1935"); err != nil {
// 		glog.Errorf("rtmpServer.Server failed, %v", err)
// 	}
// }

// func startHTTPFlv(stream *protocol.StreamHandler) {
// 	flvListen, err := net.Listen("tcp", *httpFlvAddr)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	hdlServer := httpflv.NewServer(stream)
// 	go func() {
// 		defer func() {
// 			if r := recover(); r != nil {
// 				glog.Infof("HTTP-FLV server panic: %v", r)
// 			}
// 		}()
// 		glog.Infof("HTTP-FLV listen On:%s", *httpFlvAddr)
// 		hdlServer.Serve(flvListen)
// 	}()
// }

// func startHTTPOpera(stream *protocol.StreamHandler) {
// 	if *operaAddr != "" {
// 		opListen, err := net.Listen("tcp", *operaAddr)
// 		if err != nil {
// 			glog.Errorf("net.Listen failed, %v", err)
// 			return
// 		}
// 		opServer := httpopera.NewServer(stream, *rtmpAddr)
// 		go func() {
// 			defer func() {
// 				if r := recover(); r != nil {
// 					glog.Errorf("HTTP-Operation server panic: %v", r)
// 				}
// 			}()
// 			glog.Infof("HTTP-Operation listen On:%s", *operaAddr)
// 			opServer.Serve(opListen)
// 		}()
// 	}
// }

// func main() {
// 	defer func() {
// 		if r := recover(); r != nil {
// 			fmt.Println("livego panic: %v", r)
// 			time.Sleep(1 * time.Second)
// 		}
// 	}()
// 	// err := configure.LoadConfig(*configfilename)
// 	// if err != nil {
// 	// 	return
// 	// }

// 	stream := protocol.NewRtmpStream()
// 	hlsServer := startHls()
// 	startHTTPFlv(stream)
// 	startHTTPOpera(stream)

// 	startRtmp(stream, hlsServer)
// 	//startRtmp(stream, nil)
// }

func main() {
	port := flag.Int("port", 1935, "rtmp server port")
	flag.Parse()

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("rtmp server panic:", r)
			time.Sleep(time.Second * 1)
		}
	}()
	api := srtmp.NewAPI()
	addr := fmt.Sprintf(":%d", *port)
	if err := api.ServeRtmp(addr); err != nil {
		fmt.Println("Servr rtmp failed, err:", err)
		return
	}
}
