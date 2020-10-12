package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/fabo871218/srtmp/av"
	"github.com/fabo871218/srtmp/logger"
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

//ConnectInfo ...
type ConnectInfo struct {
	App            string `amf:"app" json:"app"`
	Flashver       string `amf:"flashVer" json:"flashVer"`
	SwfURL         string `amf:"swfUrl" json:"swfUrl"`
	TcURL          string `amf:"tcUrl" json:"tcUrl"`
	Fpad           bool   `amf:"fpad" json:"fpad"`
	AudioCodecs    int    `amf:"audioCodecs" json:"audioCodecs"`
	VideoCodecs    int    `amf:"videoCodecs" json:"videoCodecs"`
	VideoFunction  int    `amf:"videoFunction" json:"videoFunction"`
	PageURL        string `amf:"pageUrl" json:"pageUrl"`
	ObjectEncoding int    `amf:"objectEncoding" json:"objectEncoding"`
}

//ConnectResp ...
type ConnectResp struct {
	FMSVer       string `amf:"fmsVer"`
	Capabilities int    `amf:"capabilities"`
}

//ConnectEvent ...
type ConnectEvent struct {
	Level          string `amf:"level"`
	Code           string `amf:"code"`
	Description    string `amf:"description"`
	ObjectEncoding int    `amf:"objectEncoding"`
}

//PublishInfo ...
type PublishInfo struct {
	Name string
	Type string
}

//ClientConn 与客户端对应的rtmp连接
type ClientConn struct {
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
	logger        logger.Logger
}

//NewClientConn 创建一个与客户端对应的rtmp连接
func NewClientConn(conn *RtmpConn, log logger.Logger) *ClientConn {
	return &ClientConn{
		conn:     conn,
		streamID: 1,
		bytesw:   bytes.NewBuffer(nil),
		decoder:  &amf.Decoder{},
		encoder:  &amf.Encoder{},
		logger:   log,
	}
}

func (client *ClientConn) writeMsg(csid, streamID uint32, args ...interface{}) error {
	client.bytesw.Reset()
	for _, v := range args {
		if _, err := client.encoder.Encode(client.bytesw, v, amf.AMF0); err != nil {
			return err
		}
	}
	msg := client.bytesw.Bytes()
	c := ChunkStream{
		Format:    0,
		CSID:      csid,
		Timestamp: 0,
		TypeID:    20,
		StreamID:  streamID,
		Length:    uint32(len(msg)),
		Data:      msg,
	}
	client.conn.Write(&c)
	return client.conn.Flush()
}

func (client *ClientConn) connect(vs []interface{}) error {
	for _, v := range vs {
		switch v.(type) {
		case string:
		case float64:
			id := int(v.(float64))
			if id != 1 {
				return ErrReq
			}
			client.transactionID = id
		case amf.Object:
			obimap := v.(amf.Object)
			if app, ok := obimap["app"]; ok {
				client.ConnInfo.App = app.(string)
			}
			if flashVer, ok := obimap["flashVer"]; ok {
				client.ConnInfo.Flashver = flashVer.(string)
			}
			if tcurl, ok := obimap["tcUrl"]; ok {
				client.ConnInfo.TcURL = tcurl.(string)
			}
			if encoding, ok := obimap["objectEncoding"]; ok {
				client.ConnInfo.ObjectEncoding = int(encoding.(float64))
			}
		}
	}
	return nil
}

func (client *ClientConn) releaseStream(vs []interface{}) error {
	return nil
}

func (client *ClientConn) fcPublish(vs []interface{}) error {
	return nil
}

//todo 参数是否要定死
func (client *ClientConn) connectResp(cur *ChunkStream) error {
	c := client.conn.NewWindowAckSize(2500000)
	client.conn.Write(&c)
	c = client.conn.NewSetPeerBandwidth(2500000)
	client.conn.Write(&c)
	c = client.conn.NewSetChunkSize(uint32(1024))
	client.conn.Write(&c)

	resp := make(amf.Object)
	resp["fmsVer"] = "FMS/3,0,1,123"
	resp["capabilities"] = 31

	event := make(amf.Object)
	event["level"] = "status"
	event["code"] = "NetConnection.Connect.Success"
	event["description"] = "Connection succeeded."
	event["objectEncoding"] = client.ConnInfo.ObjectEncoding
	return client.writeMsg(cur.CSID, cur.StreamID, "_result", client.transactionID, resp, event)
}

func (client *ClientConn) createStream(vs []interface{}) error {
	for _, v := range vs {
		switch v.(type) {
		case string:
		case float64:
			client.transactionID = int(v.(float64))
		case amf.Object:
		}
	}
	return nil
}

func (client *ClientConn) createStreamResp(cur *ChunkStream) error {
	return client.writeMsg(cur.CSID, cur.StreamID, "_result", client.transactionID, nil, client.streamID)
}

func (client *ClientConn) publishOrPlay(vs []interface{}) error {
	for k, v := range vs {
		switch v.(type) {
		case string:
			if k == 2 {
				client.PublishInfo.Name = v.(string)
			} else if k == 3 {
				client.PublishInfo.Type = v.(string)
			}
		case float64:
			id := int(v.(float64))
			client.transactionID = id
		case amf.Object:
		}
	}

	return nil
}

func (client *ClientConn) publishResp(cur *ChunkStream) error {
	event := make(amf.Object)
	event["level"] = "status"
	event["code"] = "NetStream.Publish.Start"
	event["description"] = "Start publising."
	return client.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event)
}

func (client *ClientConn) playResp(cur *ChunkStream) error {
	client.conn.SetRecorded()
	client.conn.SetBegin()

	event := make(amf.Object)
	event["level"] = "status"
	event["code"] = "NetStream.Play.Reset"
	event["description"] = "Playing and resetting stream."
	if err := client.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}

	event["level"] = "status"
	event["code"] = "NetStream.Play.Start"
	event["description"] = "Started playing stream."
	if err := client.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}

	event["level"] = "status"
	event["code"] = "NetStream.Data.Start"
	event["description"] = "Started playing stream."
	if err := client.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}

	event["level"] = "status"
	event["code"] = "NetStream.Play.PublishNotify"
	event["description"] = "Started playing notify."
	if err := client.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}
	return client.conn.Flush()
}

func (client *ClientConn) handleCmdMsg(c *ChunkStream) error {
	amfType := amf.AMF0
	if c.TypeID == 17 {
		c.Data = c.Data[1:]
	}
	r := bytes.NewReader(c.Data)
	vs, err := client.decoder.DecodeBatch(r, amf.Version(amfType))
	if err != nil && err != io.EOF {
		return err
	}
	// glog.Infof("rtmp req: %#v", vs)
	switch vs[0].(type) {
	case string:
		switch vs[0].(string) {
		case cmdConnect:
			if err = client.connect(vs[1:]); err != nil {
				return err
			}
			if err = client.connectResp(c); err != nil {
				return err
			}
		case cmdCreateStream:
			if err = client.createStream(vs[1:]); err != nil {
				return err
			}
			if err = client.createStreamResp(c); err != nil {
				return err
			}
		case cmdPublish:
			if err = client.publishOrPlay(vs[1:]); err != nil {
				return err
			}
			if err = client.publishResp(c); err != nil {
				return err
			}
			client.done = true
			client.isPublisher = true
		case cmdPlay:
			if err = client.publishOrPlay(vs[1:]); err != nil {
				return err
			}
			if err = client.playResp(c); err != nil {
				return err
			}
			client.done = true
			client.isPublisher = false
			fmt.Printf("handle play req done\n")
		case cmdFcpublish:
			client.fcPublish(vs)
		case cmdReleaseStream:
			client.releaseStream(vs)
		case cmdFCUnpublish:
		case cmdDeleteStream:
		default:
			client.logger.Warnf("no support command:%s", vs[0].(string))
		}
	}

	return nil
}

//SetUpPlayOrPublish 等待客户端完成推流或拉流请求
//todo 需要增加超时，防止连接一直在却不发送任何消息
func (client *ClientConn) SetUpPlayOrPublish() error {
	amfType := amf.AMF0
	for {
		chunk, err := client.conn.Read()
		if err != nil {
			return fmt.Errorf("Read chunk stream failed, %v", err)
		}
		//todo 需要注释一下， 20，17代表什么消息类型
		if chunk.TypeID != 17 && chunk.TypeID != 20 {
			continue
		} else if chunk.TypeID == 17 {
			chunk.Data = chunk.Data[1:]
		}

		r := bytes.NewReader(chunk.Data)
		vs, err := client.decoder.DecodeBatch(r, amf.Version(amfType))
		if err != nil && err != io.EOF {
			return fmt.Errorf("Amf DecodeBatch failed, %v", err)
		}

		if cmd, ok := vs[0].(string); ok {
			switch cmd {
			case cmdConnect:
				if err = client.connect(vs[1:]); err != nil {
					return fmt.Errorf("handle connect cmd failed, %v", err)
				}
				if err = client.connectResp(chunk); err != nil {
					return fmt.Errorf("connect response failed, %v", err)
				}
			case cmdCreateStream:
				if err = client.createStream(vs[1:]); err != nil {
					return fmt.Errorf("handle create stream cmd failed, %v", err)
				}
				if err = client.createStreamResp(chunk); err != nil {
					return fmt.Errorf("create stream response failed, %v", err)
				}
			case cmdPublish:
				if err = client.publishOrPlay(vs[1:]); err != nil {
					return fmt.Errorf("handle publish command failed, %v", err)
				}
				if err = client.publishResp(chunk); err != nil {
					return fmt.Errorf("publish response failed, %v", err)
				}
				client.isPublisher = true
				return nil
			case cmdPlay:
				if err = client.publishOrPlay(vs[1:]); err != nil {
					return fmt.Errorf("handle play command failed, %v", err)
				}
				if err = client.playResp(chunk); err != nil {
					return fmt.Errorf("play response failed, %v", err)
				}
				client.isPublisher = false
				return nil
			case cmdFcpublish:
				client.fcPublish(vs)
			case cmdReleaseStream:
				client.releaseStream(vs)
			case cmdFCUnpublish:
			case cmdDeleteStream:
			default:
				client.logger.Warnf("no support command:%s", cmd)
			}
		}
	}
}

//ReadMsg is a method
func (client *ClientConn) ReadMsg() error {
	for {
		c, err := client.conn.Read()
		if err != nil {
			return err
		}
		switch c.TypeID {
		case 20, 17:
			if err := client.handleCmdMsg(c); err != nil {
				return err
			}
		}
		if client.done {
			break
		}
	}
	return nil
}

//IsPublisher ...
func (client *ClientConn) IsPublisher() bool {
	return client.isPublisher
}

//Write ...
func (client *ClientConn) Write(c ChunkStream) error {
	if c.TypeID == av.TAG_SCRIPTDATAAMF0 ||
		c.TypeID == av.TAG_SCRIPTDATAAMF3 {
		var err error
		if c.Data, err = amf.MetaDataReform(c.Data, amf.DEL); err != nil {
			return err
		}
		c.Length = uint32(len(c.Data))
	}
	return client.conn.Write(&c)
}

//Flush ...
func (client *ClientConn) Flush() error {
	return client.conn.Flush()
}

//Read ...
func (client *ClientConn) Read() (*ChunkStream, error) {
	return client.conn.Read()
}

//GetStreamInfo ...
func (client *ClientConn) GetStreamInfo() (app string, name string, url string) {
	app = client.ConnInfo.App
	name = client.PublishInfo.Name
	url = client.ConnInfo.TcURL + "/" + client.PublishInfo.Name
	return
}

//Close ...
func (client *ClientConn) Close() {
	client.conn.Close()
}
