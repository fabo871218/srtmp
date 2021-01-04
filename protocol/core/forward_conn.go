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

//ForwardConnect 与客户端对应的rtmp连接
type ForwardConnect struct {
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

//NewForwardConnect 创建一个与客户端对应的rtmp连接
func NewForwardConnect(conn *RtmpConn, log logger.Logger) *ForwardConnect {
	return &ForwardConnect{
		conn:     conn,
		streamID: 1, //todo
		bytesw:   bytes.NewBuffer(nil),
		decoder:  &amf.Decoder{},
		encoder:  &amf.Encoder{},
		logger:   log,
	}
}

func (fc *ForwardConnect) writeMsg(csid, streamID uint32, args ...interface{}) error {
	fc.bytesw.Reset()
	for _, v := range args {
		if _, err := fc.encoder.Encode(fc.bytesw, v, amf.AMF0); err != nil {
			return err
		}
	}
	msg := fc.bytesw.Bytes()
	c := ChunkStream{
		Format:    0,
		CSID:      csid,
		Timestamp: 0,
		TypeID:    20,
		StreamID:  streamID,
		Length:    uint32(len(msg)),
		Data:      msg,
	}
	fc.conn.Write(&c)
	return fc.conn.Flush()
}

func (fc *ForwardConnect) connect(vs []interface{}) error {
	for _, v := range vs {
		switch v.(type) {
		case string:
		case float64:
			id := int(v.(float64))
			if id != 1 {
				return ErrReq
			}
			fc.transactionID = id
		case amf.Object:
			obimap := v.(amf.Object)
			if app, ok := obimap["app"]; ok {
				fc.ConnInfo.App = app.(string)
			}
			if flashVer, ok := obimap["flashVer"]; ok {
				fc.ConnInfo.Flashver = flashVer.(string)
			}
			if tcurl, ok := obimap["tcUrl"]; ok {
				fc.ConnInfo.TcURL = tcurl.(string)
			}
			if encoding, ok := obimap["objectEncoding"]; ok {
				fc.ConnInfo.ObjectEncoding = int(encoding.(float64))
			}
		}
	}
	return nil
}

func (fc *ForwardConnect) releaseStream(vs []interface{}) error {
	return nil
}

func (fc *ForwardConnect) fcPublish(vs []interface{}) error {
	return nil
}

//todo 参数是否要定死
func (fc *ForwardConnect) connectResp(cur *ChunkStream) error {
	c := fc.conn.NewWindowAckSize(2500000)
	fc.conn.Write(&c)
	c = fc.conn.NewSetPeerBandwidth(2500000)
	fc.conn.Write(&c)
	c = fc.conn.NewSetChunkSize(uint32(1024))
	fc.conn.Write(&c)

	resp := make(amf.Object)
	resp["fmsVer"] = "FMS/3,0,1,123"
	resp["capabilities"] = 31

	event := make(amf.Object)
	event["level"] = "status"
	event["code"] = "NetConnection.Connect.Success"
	event["description"] = "Connection succeeded."
	event["objectEncoding"] = fc.ConnInfo.ObjectEncoding
	return fc.writeMsg(cur.CSID, cur.StreamID, "_result", fc.transactionID, resp, event)
}

func (fc *ForwardConnect) createStream(vs []interface{}) error {
	for _, v := range vs {
		switch v.(type) {
		case string:
		case float64:
			fc.transactionID = int(v.(float64))
		case amf.Object:
		}
	}
	return nil
}

func (fc *ForwardConnect) createStreamResp(cur *ChunkStream) error {
	return fc.writeMsg(cur.CSID, cur.StreamID, "_result", fc.transactionID, nil, fc.streamID)
}

func (fc *ForwardConnect) publishOrPlay(vs []interface{}) error {
	for k, v := range vs {
		switch v.(type) {
		case string:
			if k == 2 {
				fc.PublishInfo.Name = v.(string)
			} else if k == 3 {
				fc.PublishInfo.Type = v.(string)
			}
		case float64:
			id := int(v.(float64))
			fc.transactionID = id
		case amf.Object:
		}
	}

	return nil
}

func (fc *ForwardConnect) publishResp(cur *ChunkStream) error {
	event := make(amf.Object)
	event["level"] = "status"
	event["code"] = "NetStream.Publish.Start"
	event["description"] = "Start publising."
	return fc.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event)
}

func (fc *ForwardConnect) playResp(cur *ChunkStream) error {
	fc.conn.SetRecorded()
	fc.conn.SetBegin()

	event := make(amf.Object)
	event["level"] = "status"
	event["code"] = "NetStream.Play.Reset"
	event["description"] = "Playing and resetting stream."
	if err := fc.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}

	event["level"] = "status"
	event["code"] = "NetStream.Play.Start"
	event["description"] = "Started playing stream."
	if err := fc.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}

	event["level"] = "status"
	event["code"] = "NetStream.Data.Start"
	event["description"] = "Started playing stream."
	if err := fc.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}

	event["level"] = "status"
	event["code"] = "NetStream.Play.PublishNotify"
	event["description"] = "Started playing notify."
	if err := fc.writeMsg(cur.CSID, cur.StreamID, "onStatus", 0, nil, event); err != nil {
		return err
	}
	return fc.conn.Flush()
}

func (fc *ForwardConnect) handleCmdMsg(c *ChunkStream) error {
	amfType := amf.AMF0
	if c.TypeID == 17 {
		c.Data = c.Data[1:]
	}
	r := bytes.NewReader(c.Data)
	vs, err := fc.decoder.DecodeBatch(r, amf.Version(amfType))
	if err != nil && err != io.EOF {
		return err
	}
	// glog.Infof("rtmp req: %#v", vs)
	switch vs[0].(type) {
	case string:
		switch vs[0].(string) {
		case cmdConnect:
			if err = fc.connect(vs[1:]); err != nil {
				return err
			}
			if err = fc.connectResp(c); err != nil {
				return err
			}
		case cmdCreateStream:
			if err = fc.createStream(vs[1:]); err != nil {
				return err
			}
			if err = fc.createStreamResp(c); err != nil {
				return err
			}
		case cmdPublish:
			if err = fc.publishOrPlay(vs[1:]); err != nil {
				return err
			}
			if err = fc.publishResp(c); err != nil {
				return err
			}
			fc.done = true
			fc.isPublisher = true
		case cmdPlay:
			if err = fc.publishOrPlay(vs[1:]); err != nil {
				return err
			}
			if err = fc.playResp(c); err != nil {
				return err
			}
			fc.done = true
			fc.isPublisher = false
			fmt.Printf("handle play req done\n")
		case cmdFcpublish:
			fc.fcPublish(vs)
		case cmdReleaseStream:
			fc.releaseStream(vs)
		case cmdFCUnpublish:
		case cmdDeleteStream:
		default:
			fc.logger.Warnf("no support command:%s", vs[0].(string))
		}
	}

	return nil
}

//SetUpPlayOrPublish 等待客户端完成推流或拉流请求
//todo 需要增加超时，防止连接一直在却不发送任何消息
func (fc *ForwardConnect) SetUpPlayOrPublish() error {
	amfType := amf.AMF0
	for {
		chunk, err := fc.conn.Read()
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
		vs, err := fc.decoder.DecodeBatch(r, amf.Version(amfType))
		if err != nil && err != io.EOF {
			return fmt.Errorf("Amf DecodeBatch failed, %v", err)
		}

		if cmd, ok := vs[0].(string); ok {
			switch cmd {
			case cmdConnect:
				if err = fc.connect(vs[1:]); err != nil {
					return fmt.Errorf("handle connect cmd failed, %v", err)
				}
				if err = fc.connectResp(chunk); err != nil {
					return fmt.Errorf("connect response failed, %v", err)
				}
			case cmdCreateStream:
				if err = fc.createStream(vs[1:]); err != nil {
					return fmt.Errorf("handle create stream cmd failed, %v", err)
				}
				if err = fc.createStreamResp(chunk); err != nil {
					return fmt.Errorf("create stream response failed, %v", err)
				}
			case cmdPublish:
				if err = fc.publishOrPlay(vs[1:]); err != nil {
					return fmt.Errorf("handle publish command failed, %v", err)
				}
				if err = fc.publishResp(chunk); err != nil {
					return fmt.Errorf("publish response failed, %v", err)
				}
				fc.isPublisher = true
				return nil
			case cmdPlay:
				if err = fc.publishOrPlay(vs[1:]); err != nil {
					return fmt.Errorf("handle play command failed, %v", err)
				}
				if err = fc.playResp(chunk); err != nil {
					return fmt.Errorf("play response failed, %v", err)
				}
				fc.isPublisher = false
				return nil
			case cmdFcpublish:
				fc.fcPublish(vs)
			case cmdReleaseStream:
				fc.releaseStream(vs)
			case cmdFCUnpublish:
			case cmdDeleteStream:
			default:
				fc.logger.Warnf("no support command:%s", cmd)
			}
		}
	}
}

//ReadMsg is a method
func (fc *ForwardConnect) ReadMsg() error {
	for {
		c, err := fc.conn.Read()
		if err != nil {
			return err
		}
		switch c.TypeID {
		case 20, 17:
			if err := fc.handleCmdMsg(c); err != nil {
				return err
			}
		}
		if fc.done {
			break
		}
	}
	return nil
}

//IsPublisher ...
func (fc *ForwardConnect) IsPublisher() bool {
	return fc.isPublisher
}

//Write ...
func (fc *ForwardConnect) Write(c ChunkStream) error {
	if c.TypeID == av.TAG_SCRIPTDATAAMF0 ||
		c.TypeID == av.TAG_SCRIPTDATAAMF3 {
		var err error
		if c.Data, err = amf.MetaDataReform(c.Data, amf.DEL); err != nil {
			return err
		}
		c.Length = uint32(len(c.Data))
	}
	return fc.conn.Write(&c)
}

//Flush ...
func (fc *ForwardConnect) Flush() error {
	return fc.conn.Flush()
}

//Read ...
func (fc *ForwardConnect) Read() (*ChunkStream, error) {
	return fc.conn.Read()
}

//GetStreamInfo ...
func (fc *ForwardConnect) GetStreamInfo() (app string, name string, url string) {
	app = fc.ConnInfo.App
	name = fc.PublishInfo.Name
	url = fc.ConnInfo.TcURL + "/" + fc.PublishInfo.Name
	return
}

//Close ...
func (fc *ForwardConnect) Close() {
	fc.conn.Close()
}
