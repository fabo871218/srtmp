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
		transID: 1,
		bytesw:  bytes.NewBuffer(nil),
		encoder: &amf.Encoder{},
		decoder: &amf.Decoder{},
		logger:  log,
	}
}

//DecodeBatch ...
func (cc *ConnClient) DecodeBatch(r io.Reader, ver amf.Version) (ret []interface{}, err error) {
	vs, err := cc.decoder.DecodeBatch(r, ver)
	return vs, err
}

func (cc *ConnClient) readRespMsg() error {
	var err error
	var rc ChunkStream
	for {
		if err = cc.conn.Read(&rc); err != nil {
			return err
		}
		if err != nil && err != io.EOF {
			return err
		}
		switch rc.TypeID {
		case 20, 17:
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

	fmt.Printf("writeConnectMsg: connClient.transID=%d, event=%v\n", cc.transID, event)
	if err := cc.writeMsg(cmdConnect, cc.transID, event); err != nil {
		return err
	}
	return cc.readRespMsg()
}

func (cc *ConnClient) writeCreateStreamMsg() error {
	cc.transID++
	cc.curcmdName = cmdCreateStream

	fmt.Printf("writeCreateStreamMsg: connClient.transID=%d\n", cc.transID)
	if err := cc.writeMsg(cmdCreateStream, cc.transID, nil); err != nil {
		return err
	}

	for {
		err := cc.readRespMsg()
		if err == nil {
			return err
		}

		if err == errFail {
			return err
		}
	}

}

func (cc *ConnClient) writePublishMsg() error {
	cc.transID++
	cc.curcmdName = cmdPublish
	if err := cc.writeMsg(cmdPublish, cc.transID, nil, cc.title, publishLive); err != nil {
		return err
	}
	return cc.readRespMsg()
}

func (cc *ConnClient) writePlayMsg() error {
	cc.transID++
	cc.curcmdName = cmdPlay
	fmt.Printf("writePlayMsg: connClient.transID=%d, cmdPlay=%v, connClient.title=%v\n",
		cc.transID, cmdPlay, cc.title)

	if err := cc.writeMsg(cmdPlay, 0, nil, cc.title); err != nil {
		return err
	}
	return cc.readRespMsg()
}

//Start ...
func (cc *ConnClient) Start(url string, method string) error {
	var (
		err       error
		parsedURL *neturl.URL
	)
	if parsedURL, err = neturl.Parse(url); err != nil {
		return fmt.Errorf("parse url failed, %s", err.Error())
	}

	cc.url = url
	path := strings.TrimLeft(parsedURL.Path, "/")
	ps := strings.SplitN(path, "/", 2)
	if len(ps) != 2 {
		return fmt.Errorf("path err: %s", path)
	}
	cc.app = ps[0]
	cc.title = ps[1]
	cc.query = parsedURL.RawQuery
	cc.tcurl = "rtmp://" + parsedURL.Host + "/" + cc.app
	port := ":1935"
	host := parsedURL.Host
	localIP := ":0"
	var remoteIP string
	if strings.Index(host, ":") != -1 {
		host, port, err = net.SplitHostPort(host)
		if err != nil {
			return err
		}
		port = ":" + port
	}

	var ips []net.IP
	if ips, err = net.LookupIP(host); err != nil {
		return fmt.Errorf("net.LookupIP failed, %v", err)
	}
	remoteIP = ips[rand.Intn(len(ips))].String()
	if strings.Index(remoteIP, ":") == -1 {
		remoteIP += port
	}

	var local, remote *net.TCPAddr
	if local, err = net.ResolveTCPAddr("tcp", localIP); err != nil {
		return fmt.Errorf("net.ResolveTCPAddr localIP failed, %v", err)
	} else if remote, err = net.ResolveTCPAddr("tcp", remoteIP); err != nil {
		return fmt.Errorf("net.ResolveTCPAdde remoteIP failed, %v", err)
	}

	var conn *net.TCPConn
	if conn, err = net.DialTCP("tcp", local, remote); err != nil {
		return fmt.Errorf("net.DialTCP failed, %v", err)
	}

	cc.conn = NewConn(conn, 4*1024)

	cc.logger.Debug("HandsakeClient...")
	if err = cc.conn.HandshakeClient(); err != nil {
		return fmt.Errorf("HandshakeClient failed,  %v", err)
	}

	cc.logger.Debug("writeConnectMsg....")
	if err = cc.writeConnectMsg(); err != nil {
		return fmt.Errorf("writeConnectMsg failed, %v", err)
	}

	cc.logger.Debug("writeCreateStreamMsg....")
	if err = cc.writeCreateStreamMsg(); err != nil {
		return fmt.Errorf("writeCreateStreamMsg failed, %v", err)
	}

	cc.logger.Debugf("method control:%s %s %s", method, av.PUBLISH, av.PLAY)
	if method == av.PUBLISH {
		if err = cc.writePublishMsg(); err != nil {
			return fmt.Errorf("writePublishMsg failed, %v", err)
		}
	} else if method == av.PLAY {
		if err = cc.writePlayMsg(); err != nil {
			return fmt.Errorf("writePlayMsg failed, %v", err)
		}
	}
	return nil
}

func (cc *ConnClient) Write(c ChunkStream) error {
	if c.TypeID == av.TAG_SCRIPTDATAAMF0 || c.TypeID == av.TAG_SCRIPTDATAAMF3 {
		var err error
		if c.Data, err = amf.MetaDataReform(c.Data, amf.ADD); err != nil {
			return err
		}
		c.Length = uint32(len(c.Data))
	}
	return cc.conn.Write(&c)
}

func (cc *ConnClient) Flush() error {
	return cc.conn.Flush()
}

func (cc *ConnClient) Read(c *ChunkStream) (err error) {
	return cc.conn.Read(c)
}

func (cc *ConnClient) GetStreamInfo() (app string, name string, url string) {
	app = cc.app
	name = cc.title
	url = cc.url
	return
}

func (cc *ConnClient) GetStreamId() uint32 {
	return cc.streamid
}

func (cc *ConnClient) Close() {
	cc.conn.Close()
}
