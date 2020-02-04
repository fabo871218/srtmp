package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/protocol/amf"
)

var (
	publishLive   = "live"
	publishRecord = "record"
	publishAppend = "append"
)

var (
	ErrReq = errors.New("req error")
)

var (
	cmdConnect       = "connect"
	cmdFcpublish     = "FCPublish"
	cmdReleaseStream = "releaseStream"
	cmdCreateStream  = "createStream"
	cmdPublish       = "publish"
	cmdFCUnpublish   = "FCUnpublish"
	cmdDeleteStream  = "deleteStream"
	cmdPlay          = "play"
)

type ConnectInfo struct {
	App            string `amf:"app" json:"app"`
	Flashver       string `amf:"flashVer" json:"flashVer"`
	SwfUrl         string `amf:"swfUrl" json:"swfUrl"`
	TcUrl          string `amf:"tcUrl" json:"tcUrl"`
	Fpad           bool   `amf:"fpad" json:"fpad"`
	AudioCodecs    int    `amf:"audioCodecs" json:"audioCodecs"`
	VideoCodecs    int    `amf:"videoCodecs" json:"videoCodecs"`
	VideoFunction  int    `amf:"videoFunction" json:"videoFunction"`
	PageUrl        string `amf:"pageUrl" json:"pageUrl"`
	ObjectEncoding int    `amf:"objectEncoding" json:"objectEncoding"`
}

type ConnectResp struct {
	FMSVer       string `amf:"fmsVer"`
	Capabilities int    `amf:"capabilities"`
}

type ConnectEvent struct {
	Level          string `amf:"level"`
	Code           string `amf:"code"`
	Description    string `amf:"description"`
	ObjectEncoding int    `amf:"objectEncoding"`
}

type PublishInfo struct {
	Name string
	Type string
}

type ServerConn struct {
	done          bool
	streamID      int
	isPublisher   bool
	conn          *RtmpConn
	transactionID int
	ConnInfo      ConnectInfo
	PublishInfo   PublishInfo
	decoder       *amf.Decoder
	encoder       *amf.Encoder
	bytesw        *bytes.Buffer
}

func NewServerConn(conn *RtmpConn) *ServerConn {
	return &ServerConn{
		conn:     conn,
		streamID: 1,
		bytesw:   bytes.NewBuffer(nil),
		decoder:  &amf.Decoder{},
		encoder:  &amf.Encoder{},
	}
}

func (serverConn *ServerConn) writeMsg(csid, streamID uint32, args ...interface{}) error {
	serverConn.bytesw.Reset()
	for _, v := range args {
		if _, err := serverConn.encoder.Encode(serverConn.bytesw, v, amf.AMF0); err != nil {
			return err
		}
	}
	msg := serverConn.bytesw.Bytes()
	c := ChunkStream{
		Format:    0,
		CSID:      csid,
		Timestamp: 0,
		TypeID:    20,
		StreamID:  streamID,
		Length:    uint32(len(msg)),
		Data:      msg,
	}
	serverConn.conn.Write(&c)
	return serverConn.conn.Flush()
}

func (serverConn *ServerConn) connect(vs []interface{}) error {
	for _, v := range vs {
		switch v.(type) {
		case string:
		case float64:
			id := int(v.(float64))
			if id != 1 {
				return ErrReq
			}
			serverConn.transactionID = id
		case amf.Object:
			obimap := v.(amf.Object)
			if app, ok := obimap["app"]; ok {
				serverConn.ConnInfo.App = app.(string)
			}
			if flashVer, ok := obimap["flashVer"]; ok {
				serverConn.ConnInfo.Flashver = flashVer.(string)
			}
			if tcurl, ok := obimap["tcUrl"]; ok {
				serverConn.ConnInfo.TcUrl = tcurl.(string)
			}
			if encoding, ok := obimap["objectEncoding"]; ok {
				serverConn.ConnInfo.ObjectEncoding = int(encoding.(float64))
			}
		}
	}
	return nil
}

func (serverConn *ServerConn) releaseStream(vs []interface{}) error {
	return nil
}

func (serverConn *ServerConn) fcPublish(vs []interface{}) error {
	return nil
}

func (serverConn *ServerConn) connectResp(cur *ChunkStream) error {
	c := serverConn.conn.NewWindowAckSize(2500000)
	serverConn.conn.Write(&c)
	c = serverConn.conn.NewSetPeerBandwidth(2500000)
	serverConn.conn.Write(&c)
	c = serverConn.conn.NewSetChunkSize(uint32(1024))
	serverConn.conn.Write(&c)

	resp := make(amf.Object)
	resp["fmsVer"] = "FMS/3,0,1,123"
	resp["capabilities"] = 31

	event := make(amf.Object)
	event["level"] = "status"
	event["code"] = "NetConnection.Connect.Success"
	event["description"] = "Connection succeeded."
	event["objectEncoding"] = serverConn.ConnInfo.ObjectEncoding
	return serverConn.writeMsg(cur.CSID, cur.StreamID, "_result", serverConn.transactionID, resp, event)
}

func (serverConn *ServerConn) createStream(vs []interface{}) error {
	for _, v := range vs {
		switch v.(type) {
		case string:
		case float64:
			serverConn.transactionID = int(v.(float64))
		case amf.Object:
		}
	}
	return nil
}

func (serverConn *ServerConn) createStreamResp(cur *ChunkStream) error {
	return serverConn.writeMsg(cur.CSID, cur.StreamID, "_result", serverConn.transactionID, nil, serverConn.streamID)
}

func (serverConn *ServerConn) publishOrPlay(vs []interface{}) error {
	for k, v := range vs {
		switch v.(type) {
		case string:
			if k == 2 {
				serverConn.PublishInfo.Name = v.(string)
			} else if k == 3 {
				serverConn.PublishInfo.Type = v.(string)
			}
		case float64:
			id := int(v.(float64))
			serverConn.transactionID = id
		case amf.Object:
		}
	}

	return nil
}

func (serverConn *ServerConn) publishResp(cur *ChunkStream) error {
	event := make(amf.Object)
	event["level"] = "status"
	event["code"] = "NetStream.Publish.Start"
	event["description"] = "Start publising."
	return serverConn.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event)
}

func (serverConn *ServerConn) playResp(cur *ChunkStream) error {
	serverConn.conn.SetRecorded()
	serverConn.conn.SetBegin()

	event := make(amf.Object)
	event["level"] = "status"
	event["code"] = "NetStream.Play.Reset"
	event["description"] = "Playing and resetting stream."
	if err := serverConn.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}

	event["level"] = "status"
	event["code"] = "NetStream.Play.Start"
	event["description"] = "Started playing stream."
	if err := serverConn.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}

	event["level"] = "status"
	event["code"] = "NetStream.Data.Start"
	event["description"] = "Started playing stream."
	if err := serverConn.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}

	event["level"] = "status"
	event["code"] = "NetStream.Play.PublishNotify"
	event["description"] = "Started playing notify."
	if err := serverConn.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}
	return serverConn.conn.Flush()
}

func (serverConn *ServerConn) handleCmdMsg(c *ChunkStream) error {
	amfType := amf.AMF0
	if c.TypeID == 17 {
		c.Data = c.Data[1:]
	}
	r := bytes.NewReader(c.Data)
	vs, err := serverConn.decoder.DecodeBatch(r, amf.Version(amfType))
	if err != nil && err != io.EOF {
		return err
	}
	// glog.Infof("rtmp req: %#v", vs)
	switch vs[0].(type) {
	case string:
		switch vs[0].(string) {
		case cmdConnect:
			if err = serverConn.connect(vs[1:]); err != nil {
				return err
			}
			if err = serverConn.connectResp(c); err != nil {
				return err
			}
		case cmdCreateStream:
			if err = serverConn.createStream(vs[1:]); err != nil {
				return err
			}
			if err = serverConn.createStreamResp(c); err != nil {
				return err
			}
		case cmdPublish:
			if err = serverConn.publishOrPlay(vs[1:]); err != nil {
				return err
			}
			if err = serverConn.publishResp(c); err != nil {
				return err
			}
			serverConn.done = true
			serverConn.isPublisher = true
		case cmdPlay:
			if err = serverConn.publishOrPlay(vs[1:]); err != nil {
				return err
			}
			if err = serverConn.playResp(c); err != nil {
				return err
			}
			serverConn.done = true
			serverConn.isPublisher = false
			fmt.Printf("handle play req done\n")
		case cmdFcpublish:
			serverConn.fcPublish(vs)
		case cmdReleaseStream:
			serverConn.releaseStream(vs)
		case cmdFCUnpublish:
		case cmdDeleteStream:
		default:
			fmt.Printf("no support command=\n", vs[0].(string))
		}
	}

	return nil
}

//SetUpPlayOrPublish 等待客户端完成推流或拉流请求
//todo 需要增加超时，防止连接一直在却不发送任何消息
func (serverConn *ServerConn) SetUpPlayOrPublish() error {
	amfType := amf.AMF0
	var chunk ChunkStream
	for {
		if err := serverConn.conn.Read(&chunk); err != nil {
			return fmt.Errorf("Read chunk stream failed, %v", err)
		}
		//todo 需要注释一下， 20，17代表什么消息类型
		if chunk.TypeID != 17 && chunk.TypeID != 20 {
			continue
		} else if chunk.TypeID == 17 {
			chunk.Data = chunk.Data[1:]
		}

		r := bytes.NewReader(chunk.Data)
		vs, err := serverConn.decoder.DecodeBatch(r, amf.Version(amfType))
		if err != nil && err != io.EOF {
			return fmt.Errorf("Amf DecodeBatch failed, %v", err)
		}

		if cmd, ok := vs[0].(string); ok {
			switch cmd {
			case cmdConnect:
				if err = serverConn.connect(vs[1:]); err != nil {
					return fmt.Errorf("handle connect cmd failed, %v", err)
				}
				if err = serverConn.connectResp(&chunk); err != nil {
					return fmt.Errorf("connect response failed, %v", err)
				}
			case cmdCreateStream:
				if err = serverConn.createStream(vs[1:]); err != nil {
					return fmt.Errorf("handle create stream cmd failed, %v", err)
				}
				if err = serverConn.createStreamResp(&chunk); err != nil {
					return fmt.Errorf("create stream response failed, %v", err)
				}
			case cmdPublish:
				if err = serverConn.publishOrPlay(vs[1:]); err != nil {
					return fmt.Errorf("handle publish command failed, %v", err)
				}
				if err = serverConn.publishResp(&chunk); err != nil {
					return fmt.Errorf("publish response failed, %v", err)
				}
				serverConn.isPublisher = true
				return nil
			case cmdPlay:
				if err = serverConn.publishOrPlay(vs[1:]); err != nil {
					return fmt.Errorf("handle play command failed, %v", err)
				}
				if err = serverConn.playResp(&chunk); err != nil {
					return fmt.Errorf("play response failed, %v", err)
				}
				serverConn.isPublisher = false
				return nil
			case cmdFcpublish:
				serverConn.fcPublish(vs)
			case cmdReleaseStream:
				serverConn.releaseStream(vs)
			case cmdFCUnpublish:
			case cmdDeleteStream:
			default:
				fmt.Printf("no support command:%s\n", cmd)
			}
		}
	}
	return nil
}

//ReadMsg is a method
func (serverConn *ServerConn) ReadMsg() error {
	var c ChunkStream
	for {
		if err := serverConn.conn.Read(&c); err != nil {
			return err
		}
		switch c.TypeID {
		case 20, 17:
			if err := serverConn.handleCmdMsg(&c); err != nil {
				return err
			}
		}
		if serverConn.done {
			break
		}
	}
	return nil
}

func (serverConn *ServerConn) IsPublisher() bool {
	return serverConn.isPublisher
}

func (serverConn *ServerConn) Write(c ChunkStream) error {
	if c.TypeID == av.TAG_SCRIPTDATAAMF0 ||
		c.TypeID == av.TAG_SCRIPTDATAAMF3 {
		var err error
		if c.Data, err = amf.MetaDataReform(c.Data, amf.DEL); err != nil {
			return err
		}
		c.Length = uint32(len(c.Data))
	}
	return serverConn.conn.Write(&c)
}

func (serverConn *ServerConn) Flush() error {
	return serverConn.conn.Flush()
}

func (serverConn *ServerConn) Read(c *ChunkStream) (err error) {
	return serverConn.conn.Read(c)
}

func (serverConn *ServerConn) GetStreamInfo() (app string, name string, url string) {
	app = serverConn.ConnInfo.App
	name = serverConn.PublishInfo.Name
	url = serverConn.ConnInfo.TcUrl + "/" + serverConn.PublishInfo.Name
	return
}

func (serverConn *ServerConn) Close() {
	serverConn.conn.Close()
}
