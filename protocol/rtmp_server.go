package protocol

import (
	"flag"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/configure"
	"github.com/fabo871218/srtmp/container/flv"
	"github.com/fabo871218/srtmp/protocol/core"
)

const (
	maxQueueNum           = 1024
	SAVE_STATICS_INTERVAL = 5000
)

var (
	readTimeout  = flag.Int("readTimeout", 10, "read time out")
	writeTimeout = flag.Int("writeTimeout", 10, "write time out")
)

//Server rtmpfuwu
type Server struct {
	handler      av.Handler
	extendWriter av.ExtendWriter
}

//NewRtmpServer 创建一个rtmp服务
func NewRtmpServer(h av.Handler, sw av.ExtendWriter) *Server {
	return &Server{
		handler:      h,
		extendWriter: sw,
	}
}

//Serve 启动rtmp监听服务
func (s *Server) Serve(listenAddr string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("rtmp server panic:", r)
		}
	}()

	var listener net.Listener
	listener, err = net.Listen("tcp", listenAddr)
	if err != nil {
		err = fmt.Errorf("net.Listen failed, %v", err)
	}
	fmt.Printf("start rtmp server, listen on:%s\n", listenAddr)
	for {
		var netconn net.Conn
		netconn, err = listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				//如果时临时错误，sleep一段时间继续
				fmt.Printf("Accept failed, temporary error, try again...\n")
				time.Sleep(time.Millisecond * 100)
				continue
			}
			fmt.Printf("Accept failed, err:%v\n", err)
			return
		}
		rtmpConn := core.NewConn(netconn, 4*1024)
		fmt.Printf("new rtmp connect, remote:%s local:%s\n", rtmpConn.RemoteAddr().String(), rtmpConn.LocalAddr().String())
		go s.handleConn(rtmpConn)
	}
}

//ServeTLS 启动监听rtmp tls连接
func (s *Server) ServeTLS(listenAddr string, tlsCrt, tlsKey string) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("rtmps server panic:", r)
		}
	}()

	//todo 启动tls连接监听
	return nil
}

func (s *Server) handleConn(rtmpConn *core.RtmpConn) {
	var err error
	defer func() {
		if err != nil {
			rtmpConn.Close()
		}
	}()

	if err = rtmpConn.HandshakeServer(); err != nil {
		fmt.Printf("HandshakeServer failed, %v\n", err)
		return
	}
	//创建一个服务端连接
	serverConn := core.NewServerConn(rtmpConn)
	if err = serverConn.SetUpPlayOrPublish(); err != nil {
		fmt.Printf("SetUpPlayOrPublish failed, %v\n", err)
		return
	}
	//根据appname判断流是否存在
	//如果是publish，如果对应的流已经存在，则关闭，重新创建
	//如果是play，如果对应的流不存在，返回错误
	if serverConn.IsPublisher() {
		reader := NewPeerReader(serverConn)
		if err := s.handler.HandleReader(reader); err != nil {
			fmt.Printf("HandleReader failed, %v\n", err)
			return
		}

		if s.extendWriter != nil {
			writer, err := s.extendWriter.NewWriter(reader.StreamInfo())
			if err != nil {
				fmt.Printf("s.extendWriter.NewWriter failed, %v\n", err)
				return
			}
			s.handler.HandleWriter(writer)
		}
	} else {
		writer := NewPeerWriter(serverConn)
		s.handler.HandleWriter(writer)
		fmt.Printf("Handle new writer, %s\n", writer.StreamInfo().Key)
	}
	fmt.Printf("Receive new connection, publisher:%v\n", serverConn.IsPublisher())
}

func (s *Server) handleConn1(rtmpConn *core.RtmpConn) {
	var err error
	defer func() {
		if err != nil {
			rtmpConn.Close()
		}
	}()

	if err = rtmpConn.HandshakeServer(); err != nil {
		fmt.Printf("HandshakeServer failed, %v\n", err)
		return
	}

	serverConn := core.NewServerConn(rtmpConn)
	if err = serverConn.ReadMsg(); err != nil {
		fmt.Printf("handleConn read msg error, %v\n", err)
		return
	}

	appname, _, _ := serverConn.GetStreamInfo()
	if ret := configure.CheckAppName(appname); !ret {
		err = fmt.Errorf("application name=%s is not configured", appname)
		fmt.Printf("CheckAppName failed, name:%s %v\n", appname, err)
		return
	}

	fmt.Printf("handleConn: IsPublisher=%v\n", serverConn.IsPublisher())
	if serverConn.IsPublisher() {
		if pushlist, ret := configure.GetStaticPushUrlList(appname); ret && (pushlist != nil) {
			fmt.Printf("GetStaticPushUrlList: %v\n", pushlist)
		}
		reader := NewPeerReader(serverConn)
		s.handler.HandleReader(reader)
		fmt.Printf("new publisher: %+v\n", reader.StreamInfo())

		if s.extendWriter != nil {
			writeType := reflect.TypeOf(s.extendWriter)
			fmt.Printf("handleConn:writeType=%v\n", writeType)
			writer, err := s.extendWriter.NewWriter(reader.StreamInfo())
			if err != nil {
				fmt.Printf("s.extendWriter.NewWriter failed, %v\n", err)
				return
			}
			s.handler.HandleWriter(writer)
		}
		flvDvr := &flv.FlvDvr{}
		flvWriter, err := flvDvr.NewWriter(reader.StreamInfo())
		if err != nil {
			fmt.Printf("Create flv writer failed, %v\n", err)
		} else {
			s.handler.HandleWriter(flvWriter)
		}
	} else {
		writer := NewPeerWriter(serverConn)
		fmt.Printf("new player: %+v\n", writer.StreamInfo())
		s.handler.HandleWriter(writer)
	}
	return
}
