package srtmp

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol"
	"github.com/fabo871218/srtmp/protocol/core"
	"registry.code.tuya-inc.top/TuyaBEMiddleWare/golib/golog"
)

//Server rtmpfuwu
type Server struct {
	handler *protocol.StreamHandler
	logger  logger.Logger
}

//NewRtmpServer 创建一个rtmp服务
func NewRtmpServer(h *protocol.StreamHandler, log logger.Logger) *Server {
	return &Server{
		handler: h,
		logger:  log,
	}
}

//Serve 启动rtmp监听服务
func (s *Server) Serve(listenAddr string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("rtmp server panic:%v", r)
		}
	}()
	var listener net.Listener
	listener, err = net.Listen("tcp", listenAddr)
	if err != nil {
		err = fmt.Errorf("net.Listen failed, %v", err)
	}
	s.logger.Infof("Start rtmp server, listen on:%s", listenAddr)
	for {
		var netconn net.Conn
		netconn, err = listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				//如果时临时错误，sleep一段时间继续
				s.logger.Warn("Accept failed, temporary error, try again...")
				time.Sleep(time.Millisecond * 100)
				continue
			}
			s.logger.Errorf("Accept failed, err:%s", err.Error())
			return
		}
		rtmpConn := core.NewRtmpConn(netconn, 4*1024)
		s.logger.Infof("New rtmp connect, remote:%s local:%s",
			rtmpConn.RemoteAddr().String(), rtmpConn.LocalAddr().String())
		go s.handleConn(rtmpConn)
	}
}

//ServeTLS 启动监听rtmp tls连接
func (s *Server) ServeTLS(listenAddr string, tlsCrt, tlsKey string) error {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("rtmps server panic:%v", r)
		}
	}()

	cert, err := tls.LoadX509KeyPair(tlsCrt, tlsKey)
	if err != nil {
		return fmt.Errorf("tls.LoadX509KeyPair failed, %s", err.Error())
	}

	var listener net.Listener
	config := &tls.Config{Certificates: []tls.Certificate{cert}}
	listener, err = tls.Listen("tcp", listenAddr, config)
	if err != nil {
		golog.Error("Listen rtsp tls failed.", golog.String("err", err.Error()))
		return fmt.Errorf("Listen rtsp tls failed, %s", err.Error())
	}

	s.logger.Infof("Start rtmps server, listen on:%s", listenAddr)
	for {
		var netconn net.Conn
		netconn, err = listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				//如果时临时错误，sleep一段时间继续
				s.logger.Warn("Accept failed, temporary error, try again...")
				time.Sleep(time.Millisecond * 100)
				continue
			}
			return fmt.Errorf("Accept failed, %s", err.Error())
		}
		rtmpConn := core.NewRtmpConn(netconn, 4*1024)
		s.logger.Infof("New rtmp connect, remote:%s local:%s",
			rtmpConn.RemoteAddr().String(), rtmpConn.LocalAddr().String())
		go s.handleConn(rtmpConn)
	}
}

func (s *Server) handleConn(rtmpConn *core.RtmpConn) {
	var err error
	defer func() {
		if err != nil {
			rtmpConn.Close()
		}
	}()

	if err = rtmpConn.HandshakeServer(); err != nil {
		s.logger.Errorf("HandshakeServer failed, %s", err.Error())
		return
	}
	//创建一个服务端连接
	forwardConn := core.NewForwardConnect(rtmpConn, s.logger)
	if err = forwardConn.SetUpPlayOrPublish(); err != nil {
		s.logger.Errorf("SetUpPlayOrPublish failed, %s", err.Error())
		return
	}
	//根据appname判断流是否存在
	//如果是publish，如果对应的流已经存在，则关闭，重新创建
	//如果是play，如果对应的流不存在，返回错误
	if err = s.handler.HandleConnect(forwardConn); err != nil {
		s.logger.Errorf("Handle connect failed, %v", err)
		return
	}
	s.logger.Infof("Receive new connection, publisher:%v", forwardConn.IsPublisher())
}

// func (s *Server) handleConn1(rtmpConn *core.RtmpConn) {
// 	var err error
// 	defer func() {
// 		if err != nil {
// 			rtmpConn.Close()
// 		}
// 	}()

// 	if err = rtmpConn.HandshakeServer(); err != nil {
// 		fmt.Printf("HandshakeServer failed, %v\n", err)
// 		return
// 	}

// 	clientConn := core.NewClientConn(rtmpConn)
// 	if err = clientConn.ReadMsg(); err != nil {
// 		fmt.Printf("handleConn read msg error, %v\n", err)
// 		return
// 	}

// 	appname, _, _ := clientConn.GetStreamInfo()
// 	if ret := configure.CheckAppName(appname); !ret {
// 		err = fmt.Errorf("application name=%s is not configured", appname)
// 		s.logger.Errorf("CheckAppName failed, name:%s %v", appname, err)
// 		return
// 	}

// 	fmt.Printf("handleConn: IsPublisher=%v\n", clientConn.IsPublisher())
// 	if clientConn.IsPublisher() {
// 		if pushlist, ret := configure.GetStaticPushUrlList(appname); ret && (pushlist != nil) {
// 			s.logger.Infof("GetStaticPushUrlList: %v", pushlist)
// 		}
// 		reader := protocol.NewStreamReader(clientConn, s.logger)
// 		s.handler.HandleReader(reader)
// 		fmt.Printf("new publisher: %+v\n", reader.StreamInfo())

// 		if s.extendWriter != nil {
// 			writeType := reflect.TypeOf(s.extendWriter)
// 			fmt.Printf("handleConn:writeType=%v\n", writeType)
// 			writer, err := s.extendWriter.NewWriter(reader.StreamInfo())
// 			if err != nil {
// 				fmt.Printf("s.extendWriter.NewWriter failed, %v\n", err)
// 				return
// 			}
// 			s.handler.HandleWriter(writer)
// 		}
// 		flvDvr := &flv.FlvDvr{}
// 		flvWriter, err := flvDvr.NewWriter(reader.StreamInfo())
// 		if err != nil {
// 			fmt.Printf("Create flv writer failed, %v\n", err)
// 		} else {
// 			s.handler.HandleWriter(flvWriter)
// 		}
// 	} else {
// 		writer := protocol.NewStreamWriter(clientConn, s.logger)
// 		fmt.Printf("new player: %+v\n", writer.StreamInfo())
// 		s.handler.HandleWriter(writer)
// 	}
// 	return
// }
