package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	neturl "net/url"
	"strings"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/logger"
	"github.com/fabo871218/srtmp/protocol/amf"
)

var (
	respResult     = "_result"
	respError      = "_error"
	onStatus       = "onStatus"
	publishStart   = "NetStream.Publish.Start"
	playStart      = "NetStream.Play.Start"
	playReset      = "NetStream.Play.Reset"
	connectSuccess = "NetConnection.Connect.Success"
	onBWDone       = "onBWDone"
)

var (
	errFail = errors.New("respone err")
)

//ConnClient ...
type ConnClient struct {
	done       bool
	transID    int
	url        string
	tcurl      string
	app        string
	title      string
	query      string
	curcmdName string
	streamid   uint32
	conn       *RtmpConn
	encoder    *amf.Encoder
	decoder    *amf.Decoder
	bytesw     *bytes.Buffer
	logger     logger.Logger
}

//NewConnClient ...
func NewConnClient(log logger.Logger) *ConnClient {
	return &ConnClient{
		transID: 1, //todo 写死？
		bytesw:  bytes.NewBuffer(nil),
		encoder: &amf.Encoder{},
		decoder: &amf.Decoder{},
		logger:  log,
	}
}

//DecodeBatch ...
func (cc *ConnClient) DecodeBatch(r io.Reader, ver amf.Version) (ret []interface{}, err error) {
	return cc.decoder.DecodeBatch(r, ver)
}

func (cc *ConnClient) Decode(r io.Reader, ver amf.Version) (interface{}, error) {
	return cc.decoder.Decode(r, ver)
}

//todo 需要完善，功能不完整
func (cc *ConnClient) waitForResponse(commandName string) error {
	for {
		//读取一个完整的一个message
		cs, err := cc.conn.Read()
		if err != nil {
			return fmt.Errorf("read chunk stream failed, %v", err)
		}
		switch cs.TypeID {
		case 18, 15: //数据消息,传递一些元数据 amf0-18, amf3-15
			//理论上不应该出现数据消息
			return errors.New("metadata message should not received")
		case 19, 16: //共享对象消息, afm0-19, afm3-16
			//忽略共享消息？？
			cc.logger.Warn("shared message received.")
			continue
		case 8, 9: //音视频消息, 8-音频数据  9-视频数据
			//不应该出现音视频消息
			return errors.New("video and audio message should not received")
		case 22: //组合消息
			//忽略组合消息？？
			cc.logger.Warn("aggregage message received.")
		case 4: //用户控制消息
			//发送connect后，会接收到用户控制消息，比如Stream Begin
			//todo 如何解析用户消息
			cc.logger.Warn("user control message received.")
			continue //忽略该消息
		case 20, 17: //控制消息 amf0-20, amf3-17
			var vs []interface{}
			r := bytes.NewReader(cs.Data)
			if cs.TypeID == 20 {
				vs, err = cc.decoder.DecodeBatch(r, amf.AMF0)
			} else if cs.TypeID == 17 {
				vs, err = cc.decoder.DecodeBatch(r, amf.AMF3)
			}

			if err != nil && err != io.EOF {
				return fmt.Errorf("decode chunk stream failed, %v", err)
			}

			switch commandName {
			case cmdConnect:
				var bResult, bTransID, bCode bool
				for k, v := range vs {
					if result, ok := v.(string); ok && result == respResult {
						bResult = true
					} else if transID, ok := v.(float64); ok {
						if k == 1 && int(transID) == cc.transID {
							bTransID = true
						}
					} else if objmap, ok := v.(amf.Object); ok {
						if obj, ok := objmap["code"]; ok {
							if code, ok := obj.(string); ok && code == connectSuccess {
								bCode = true
							}
						}
					}
				}
				if !bResult || !bTransID || !bCode {
					return fmt.Errorf("result:%v transID:%v code:%v", bResult, bTransID, bCode)
				}
			case cmdCreateStream:
				var bResult, bTransID bool
				for k, v := range vs {
					if result, ok := v.(string); ok && result == respResult {
						bResult = true
					} else if id, ok := v.(float64); ok {
						if k == 1 && int(id) == cc.transID {
							bTransID = true
						}
						if k == 3 {
							cc.streamid = uint32(id)
						}
					}
				}
				if !bResult || !bTransID {
					return fmt.Errorf("result:%v transID:%v", bResult, bTransID)
				}
			case cmdPlay:
				var bResult, bStart, bReset bool
				for _, v := range vs {
					if result, ok := v.(string); ok && result == onStatus {
						bResult = true
					} else if objmap, ok := v.(amf.Object); ok {
						if obj, ok := objmap["code"]; ok {
							if code, ok := obj.(string); ok {
								if code == playReset {
									bReset = true
								} else if code == playStart {
									bStart = true
								}
							}
						}
					}
				}
				if !bResult || (!bStart && !bReset) {
					return fmt.Errorf("result:%v start:%v reset:%v", bResult, bStart, bReset)
				}
			case cmdPublish:
				var bResult, bStart bool
				for _, v := range vs {
					if result, ok := v.(string); ok && result == onStatus {
						bResult = true
					} else if objmap, ok := v.(amf.Object); ok {
						if obj, ok := objmap["code"]; ok {
							if code, ok := obj.(string); ok && code == publishStart {
								bStart = true
							}
						}
					}
				}
				if !bResult || !bStart {
					return fmt.Errorf("result:%v code:%v", bResult, bStart)
				}
			default:
				return fmt.Errorf("unknow command:%s", commandName)
			}
		}
		return nil
	}
}

func (cc *ConnClient) readRespMsg() error {
	for {
		//读取一个chunk
		rc, err := cc.conn.Read()
		if err != nil {
			if err != io.EOF {
				return err
			}
		}
		switch rc.TypeID {
		case 20, 17: //如果是控制消息
			r := bytes.NewReader(rc.Data)
			vs, _ := cc.decoder.DecodeBatch(r, amf.AMF0)

			for k, v := range vs {
				switch v.(type) {
				case string:
					switch cc.curcmdName {
					case cmdConnect, cmdCreateStream:
						if v.(string) != respResult {
							return errors.New(v.(string))
						}
					case cmdPublish:
						if v.(string) != onStatus {
							return errFail
						}
					}
				case float64:
					switch cc.curcmdName {
					case cmdConnect, cmdCreateStream:
						id := int(v.(float64))

						if k == 1 {
							if id != cc.transID {
								return errFail
							}
						} else if k == 3 {
							cc.streamid = uint32(id)
						}
					case cmdPublish:
						if int(v.(float64)) != 0 {
							return errFail
						}
					}
				case amf.Object:
					objmap := v.(amf.Object)
					switch cc.curcmdName {
					case cmdConnect:
						code, ok := objmap["code"]
						if ok && code.(string) != connectSuccess {
							return errFail
						}
					case cmdPublish:
						code, ok := objmap["code"]
						if ok && code.(string) != publishStart {
							return errFail
						}
					}
				}
			}
			return nil
		}
	}
}

func (cc *ConnClient) writeMsg(args ...interface{}) error {
	cc.bytesw.Reset()
	for _, v := range args {
		if _, err := cc.encoder.Encode(cc.bytesw, v, amf.AMF0); err != nil {
			return err
		}
	}
	msg := cc.bytesw.Bytes()
	c := ChunkStream{
		Format:    0,
		CSID:      3,
		Timestamp: 0,
		TypeID:    20,
		StreamID:  cc.streamid,
		Length:    uint32(len(msg)),
		Data:      msg,
	}
	cc.conn.Write(&c)
	return cc.conn.Flush()
}

func (cc *ConnClient) writeConnectMsg() error {
	event := make(amf.Object)
	event["app"] = cc.app
	event["type"] = "nonprivate"
	event["flashVer"] = "FMS.3.1"
	event["tcUrl"] = cc.tcurl
	cc.curcmdName = cmdConnect

	cc.logger.Tracef("writeConnectMsg: connClient.transID=%d, event=%v", cc.transID, event)
	if err := cc.writeMsg(cmdConnect, cc.transID, event); err != nil {
		return err
	}
	return nil
}

func (cc *ConnClient) writeCreateStreamMsg() error {
	cc.transID++
	cc.curcmdName = cmdCreateStream

	cc.logger.Tracef("writeCreateStreamMsg: connClient.transID=%d", cc.transID)
	if err := cc.writeMsg(cmdCreateStream, cc.transID, nil); err != nil {
		return err
	}
	return nil
}

func (cc *ConnClient) writePublishMsg() error {
	cc.transID++
	cc.curcmdName = cmdPublish
	if err := cc.writeMsg(cmdPublish, cc.transID, nil, cc.title, publishLive); err != nil {
		return err
	}
	return nil
}

func (cc *ConnClient) writePlayMsg() error {
	cc.transID++
	cc.curcmdName = cmdPlay
	cc.logger.Tracef("writePlayMsg: connClient.transID=%d, cmdPlay=%v, connClient.title=%v",
		cc.transID, cmdPlay, cc.title)

	if err := cc.writeMsg(cmdPlay, 0, nil, cc.title); err != nil {
		return err
	}
	return nil
}

func (cc *ConnClient) parseURL(url string) (local, remote string, err error) {
	var parsedURL *neturl.URL
	if parsedURL, err = neturl.Parse(url); err != nil {
		err = fmt.Errorf("parse url failed, %v", err)
		return
	}

	cc.url = url
	path := strings.TrimLeft(parsedURL.Path, "/")
	ps := strings.SplitN(path, "/", 2)
	if len(ps) != 2 {
		err = fmt.Errorf("path err, %s", path)
		return
	}
	cc.app = ps[0]
	cc.title = ps[1]
	cc.query = parsedURL.RawQuery
	cc.tcurl = "rtmp://" + parsedURL.Host + "/" + cc.app
	port := ":1935"
	host := parsedURL.Host
	local = ":0"
	if strings.Index(host, ":") != -1 {
		host, port, err = net.SplitHostPort(host)
		if err != nil {
			return
		}
		port = ":" + port
	}

	var ips []net.IP
	if ips, err = net.LookupIP(host); err != nil {
		err = fmt.Errorf("net.LookupIP failed, %v", err)
		return
	}
	remote = ips[rand.Intn(len(ips))].String()
	if strings.Index(remote, ":") == -1 {
		remote += port
	}
	return
}

func (cc *ConnClient) connectServer(url string) error {
	localIP, remoteIP, err := cc.parseURL(url)
	if err != nil {
		return fmt.Errorf("parse url:%s faile, %v", url, err)
	}

	var localAddr, remoteAddr *net.TCPAddr
	if localAddr, err = net.ResolveTCPAddr("tcp", localIP); err != nil {
		return fmt.Errorf("net.ResolveTCPAddr localIP failed, %v", err)
	} else if remoteAddr, err = net.ResolveTCPAddr("tcp", remoteIP); err != nil {
		return fmt.Errorf("net.ResolveTCPAddr remoteIP failed, %v", err)
	}

	var conn *net.TCPConn
	if conn, err = net.DialTCP("tcp", localAddr, remoteAddr); err != nil {
		return fmt.Errorf("net.DialTCP failed, %v", err)
	}

	rtmpConn := NewRtmpConn(conn, 4*1024)
	defer func() {
		if err != nil {
			rtmpConn.Close()
		}
	}()

	cc.logger.Debug("HandsakeClient...")
	if err = rtmpConn.HandshakeClient(); err != nil {
		return fmt.Errorf("HandshakeClient failed,  %v", err)
	}
	cc.conn = rtmpConn
	return nil
}

func (cc *ConnClient) checkResponse(commandName string, values []interface{}) error {
	var resultOK bool = false
	for k, v := range values {
		switch v.(type) {
		case string:
			if commandName == cmdConnect || commandName == cmdCreateStream {
				if v.(string) != respResult {
					return errors.New(v.(string))
				}
				resultOK = true
			} else if commandName == cmdPublish {
				if v.(string) != onStatus {
					return errFail
				}
				resultOK = true
			}
		case float64:
			if commandName == cmdConnect || commandName == cmdCreateStream {
				id := int(v.(float64))
				if k == 1 {
					if id != cc.transID {
						return errFail
					}
				} else if k == 3 {
					cc.streamid = uint32(id)
				}
			} else if commandName == cmdPublish {
				if int(v.(float64)) != 0 {
					return errFail
				}
			}
		case amf.Object:
			objmap := v.(amf.Object)
			if commandName == cmdConnect {
				if code, ok := objmap["code"]; ok && code.(string) != connectSuccess {
					return errFail
				}
			} else if commandName == cmdPublish {
				if code, ok := objmap["code"]; ok && code.(string) != publishStart {
					return errFail
				}
			}
		}
	}

	if !resultOK {
		return fmt.Errorf("check result failed")
	}
	return nil
}

//netConnection 建立网络连接
//先发送connect消息，然后等待connect response
func (cc *ConnClient) netConnection() (err error) {
	if err := cc.writeConnectMsg(); err != nil {
		return fmt.Errorf("write connect message failed, %v", err)
	}
	var cs *ChunkStream
	for {
		if cs, err = cc.conn.Read(); err != nil {
			return fmt.Errorf("read chunk stream failed, %v", err)
		}

		switch cs.TypeID {
		case 20, 17: //指令消息, amf0-20, amf3-17
			//处理connect消息响应
			var vs []interface{}
			r := bytes.NewReader(cs.Data)
			if cs.TypeID == 20 {
				vs, err = cc.decoder.DecodeBatch(r, amf.AMF0)
			} else if cs.TypeID == 17 {
				vs, err = cc.decoder.DecodeBatch(r, amf.AMF3)
			}
			if err != nil && err != io.EOF {
				return fmt.Errorf("decode chunk stream failed, %v", err)
			}
			cc.logger.Tracef("connect response:%v", vs)
			if err = cc.checkResponse(cmdConnect, vs); err != nil {
				return fmt.Errorf("check connect response failed, %v", err)
			}
			cc.logger.Trace("check connect response success")
			//如果校验通过，就返回
			return
		case 18, 15: //数据消息,传递一些元数据 amf0-18, amf3-15
			//理论上不应该出现数据消息
			return errors.New("metadata message should not received")
		case 19, 16: //共享对象消息, afm0-19, afm3-16
			//忽略共享消息？？
			cc.logger.Warn("shared message received.")
		case 8, 9: //音视频消息, 8-音频数据  9-视频数据
			//不应该出现音视频消息
			return errors.New("video and audio message should not received")
		case 22: //组合消息
			//忽略组合消息？？
			cc.logger.Warn("aggregage message received.")
		case 4: //用户控制消息
			//发送connect后，会接收到用户控制消息，比如Stream Begin
			//todo 如何解析用户消息
			cc.logger.Warn("user control message received.")
		}
	}
}

//streamConnection 建立流连接
func (cc *ConnClient) streamConnection() (err error) {
	if err = cc.writeCreateStreamMsg(); err != nil {
		return fmt.Errorf("write create stream failed, %v", err)
	}
	var cs *ChunkStream
	for {
		if cs, err = cc.conn.Read(); err != nil {
			return fmt.Errorf("read chunk stream failed, %v", err)
		}

		switch cs.TypeID {
		case 20, 17: //指令消息, amf0-20, amf3-17
			//处理connect消息响应
			var vs []interface{}
			r := bytes.NewReader(cs.Data)
			if cs.TypeID == 20 {
				vs, err = cc.decoder.DecodeBatch(r, amf.AMF0)
			} else if cs.TypeID == 17 {
				vs, err = cc.decoder.DecodeBatch(r, amf.AMF3)
			}
			if err != nil && err != io.EOF {
				return fmt.Errorf("decode chunk stream failed, %v", err)
			}
			cc.logger.Tracef("create stream response:%v", vs)
			if err = cc.checkResponse(cmdCreateStream, vs); err != nil {
				return fmt.Errorf("check create stream response failed, %v", err)
			}
			cc.logger.Trace("check create stream response success")
			//如果校验通过，就返回
			return
		case 18, 15: //数据消息,传递一些元数据 amf0-18, amf3-15
			//理论上不应该出现数据消息
			return errors.New("metadata message should not received")
		case 19, 16: //共享对象消息, afm0-19, afm3-16
			//忽略共享消息？？
			cc.logger.Warn("shared message received.")
		case 8, 9: //音视频消息, 8-音频数据  9-视频数据
			//不应该出现音视频消息
			return errors.New("video and audio message should not received")
		case 22: //组合消息
			//忽略组合消息？？
			cc.logger.Warn("aggregage message received.")
		case 4: //用户控制消息
			//发送connect后，会接收到用户控制消息，比如Stream Begin
			//todo 如何解析用户消息
			cc.logger.Warn("user control message received.")
		}
	}
}

func (cc *ConnClient) setupPlayOrPublish(method string) (err error) {
	if method == av.PLAY {
		err = cc.writePlayMsg()
	} else if method == av.PUBLISH {
		err = cc.writePublishMsg()
	} else {
		return fmt.Errorf("unsupport method:%s", method)
	}

	var cs *ChunkStream
	for {
		if cs, err = cc.conn.Read(); err != nil {
			return fmt.Errorf("read chunk stream failed, %v", err)
		}

		switch cs.TypeID {
		case 20, 17: //指令消息, amf0-20, amf3-17
			//处理connect消息响应
			var vs []interface{}
			r := bytes.NewReader(cs.Data)
			if cs.TypeID == 20 {
				vs, err = cc.decoder.DecodeBatch(r, amf.AMF0)
			} else if cs.TypeID == 17 {
				vs, err = cc.decoder.DecodeBatch(r, amf.AMF3)
			}
			if err != nil && err != io.EOF {
				return fmt.Errorf("decode chunk stream failed, %v", err)
			}
			cc.logger.Tracef("play response:%v", vs)
			if err = cc.checkResponse(cmdPlay, vs); err != nil {
				return fmt.Errorf("check play response failed, %v", err)
			}
			cc.logger.Trace("check play response success")
			//如果校验通过，就返回
			return
		case 18, 15: //数据消息,传递一些元数据 amf0-18, amf3-15
			//理论上不应该出现数据消息
			return errors.New("metadata message should not received")
		case 19, 16: //共享对象消息, afm0-19, afm3-16
			//忽略共享消息？？
			cc.logger.Warn("shared message received.")
		case 8, 9: //音视频消息, 8-音频数据  9-视频数据
			//不应该出现音视频消息
			return errors.New("video and audio message should not received")
		case 22: //组合消息
			//忽略组合消息？？
			cc.logger.Warn("aggregage message received.")
		case 4: //用户控制消息
			//发送connect后，会接收到用户控制消息，比如Stream Begin
			//todo 如何解析用户消息
			cc.logger.Warn("user control message received.")
		}
	}
}

//Start ...
func (cc *ConnClient) Start(url string, method string) (err error) {
	if err = cc.connectServer(url); err != nil {
		return fmt.Errorf("connect to server failed, %v", err)
	}

	curCommand := cmdConnect
	for {
		switch curCommand {
		case cmdConnect:
			if err = cc.writeConnectMsg(); err != nil {
				return fmt.Errorf("write connect msg failed, %v", err)
			}
		case cmdCreateStream:
			if err = cc.writeCreateStreamMsg(); err != nil {
				return fmt.Errorf("write create stream failed, %v", err)
			}
		case cmdPlay:
			if err = cc.writePlayMsg(); err != nil {
				return fmt.Errorf("write play msg failed, %v", err)
			}
		case cmdPublish:
			if err = cc.writePublishMsg(); err != nil {
				return fmt.Errorf("write publish msg failed, %v", err)
			}
		}

		cc.logger.Tracef("Send command:%s success", curCommand)
		if err = cc.waitForResponse(curCommand); err != nil {
			return fmt.Errorf("wait for %s response failed, %v", curCommand, err)
		}
		cc.logger.Tracef("wait for %s response success", curCommand)
		switch curCommand {
		case cmdConnect:
			curCommand = cmdCreateStream
		case cmdCreateStream:
			if method == av.PUBLISH {
				curCommand = cmdPublish
			} else if method == av.PLAY {
				curCommand = cmdPlay
			} else {
				return fmt.Errorf("unsupport method:%s", method)
			}
		case cmdPlay, cmdPublish:
			return nil
		}
	}
}

func (cc *ConnClient) Write(c *ChunkStream) error {
	if c.TypeID == av.TAG_SCRIPTDATAAMF0 || c.TypeID == av.TAG_SCRIPTDATAAMF3 {
		var err error
		if c.Data, err = amf.MetaDataReform(c.Data, amf.ADD); err != nil {
			return err
		}
		c.Length = uint32(len(c.Data))
	}
	return cc.conn.Write(c)
}

//Flush ...
func (cc *ConnClient) Flush() error {
	return cc.conn.Flush()
}

func (cc *ConnClient) Read() (*ChunkStream, error) {
	return cc.conn.Read()
}

//GetStreamInfo ...
func (cc *ConnClient) GetStreamInfo() (app string, name string, url string) {
	app = cc.app
	name = cc.title
	url = cc.url
	return
}

//GetStreamID ...
func (cc *ConnClient) GetStreamID() uint32 {
	return cc.streamid
}

//Close ...
func (cc *ConnClient) Close() {
	cc.conn.Close()
}
