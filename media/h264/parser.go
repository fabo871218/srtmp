package h264

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	i_frame byte = 0
	p_frame byte = 1
	b_frame byte = 2
)

const (
	nalu_type_not_define byte = 0
	nalu_type_slice      byte = 1  //slice_layer_without_partioning_rbsp() sliceheader
	nalu_type_dpa        byte = 2  // slice_data_partition_a_layer_rbsp( ), slice_header
	nalu_type_dpb        byte = 3  // slice_data_partition_b_layer_rbsp( )
	nalu_type_dpc        byte = 4  // slice_data_partition_c_layer_rbsp( )
	nalu_type_idr        byte = 5  // slice_layer_without_partitioning_rbsp( ),sliceheader
	nalu_type_sei        byte = 6  //sei_rbsp( )
	nalu_type_sps        byte = 7  //seq_parameter_set_rbsp( )
	nalu_type_pps        byte = 8  //pic_parameter_set_rbsp( )
	nalu_type_aud        byte = 9  // access_unit_delimiter_rbsp( )
	nalu_type_eoesq      byte = 10 //end_of_seq_rbsp( )
	nalu_type_eostream   byte = 11 //end_of_stream_rbsp( )
	nalu_type_filler     byte = 12 //filler_data_rbsp( )
)

const (
	naluBytesLen int = 4
	maxSpsPpsLen int = 2 * 1024
)

var (
	decDataNil        = errors.New("dec buf is nil")
	spsDataError      = errors.New("sps data error")
	ppsHeaderError    = errors.New("pps header error")
	ppsDataError      = errors.New("pps data error")
	naluHeaderInvalid = errors.New("nalu header invalid")
	videoDataInvalid  = errors.New("video data not match")
	dataSizeNotMatch  = errors.New("data size not match")
	naluBodyLenError  = errors.New("nalu body len error")
)

var StartCode3 = []byte{0x00, 0x00, 0x01}
var StartCode4 = []byte{0x00, 0x00, 0x00, 0x01}
var naluAud = []byte{0x00, 0x00, 0x00, 0x01, 0x09, 0xf0}

type Parser struct {
	frameType    byte
	specificInfo []byte
	pps          *bytes.Buffer
}

// AVC sequence header & AAC sequence header
/*
class AVCDecoderConfigurationRecord {
	unsigned int(8) configurationVersion = 1;
	unsigned int(8) AVCProfileIndication
	unsigned int(8) profile_compatiblity;
	unsigned int(8) AVCLevelIndication
	reserved bit(6) `111111`b;
	unsigned int(2) lengthSizeMinusOne;
	reserved bit(3) `111`b;
	unsigned int(5) numOfSequenceParameterSets;
	for (i = 0; i < numOfSequenceParameterSets; i++) {
		unsigned int(16) sequenceParameterSetLength;
		bit (8 * sequenceParameterSetLength) sequenceParameterSetNALUnit;
	}
	unsigned int (8) numOfPictureParameterSet;
	for(i=0; i < numOfPictureParameterSets; i++) {
		unsigned int(16) pictureParameterSetLength;
		bit (8 * pictureParameterSetLength) pictureParameterSetNALUnit;
	}
}
*/

type sequenceHeader struct {
	configVersion byte //8bits 固定值1
	//Baseline Profile（BP， 66） 视频会议和移动应用
	//Main Profile (MP, 77): 标清电视
	//High Profile （HiP， 100): 高清电视
	avcProfileIndication byte //8bits
	profileCompatility   byte //8bits
	//H.264的Level，通过level可指定最大的图像分辨率，帧率
	avcLevelIndication byte //8bits
	reserved1          byte //6bits `111111`
	naluLen            byte //2bits
	reserved2          byte //3bits
	spsNum             byte //5bits sps的数量，一般为1
	ppsNum             byte //8bits pps的数量，一般为1
	spsLen             int  //16bits sps长度,后面跟随spsNALUint
	ppsLen             int  //16bits pps长度，后面跟随ppsNALUint
}

func NewParser() *Parser {
	return &Parser{
		pps: bytes.NewBuffer(make([]byte, maxSpsPpsLen)),
	}
}

//return value 1:sps, value2 :pps
func (parser *Parser) parseSpecificInfo(src []byte) error {
	if len(src) < 9 {
		return decDataNil
	}
	sps := []byte{}
	pps := []byte{}

	var seq sequenceHeader
	seq.configVersion = src[0]
	seq.avcProfileIndication = src[1]
	seq.profileCompatility = src[2]
	seq.avcLevelIndication = src[3]
	seq.reserved1 = src[4] & 0xfc
	seq.naluLen = src[4]&0x03 + 1
	seq.reserved2 = src[5] >> 5

	//get sps
	seq.spsNum = src[5] & 0x1f
	seq.spsLen = int(src[6])<<8 | int(src[7])

	if len(src[8:]) < seq.spsLen || seq.spsLen <= 0 {
		return spsDataError
	}
	sps = append(sps, StartCode4...)
	sps = append(sps, src[8:(8+seq.spsLen)]...)

	//get pps
	tmpBuf := src[(8 + seq.spsLen):]
	if len(tmpBuf) < 4 {
		return ppsHeaderError
	}
	seq.ppsNum = tmpBuf[0]
	seq.ppsLen = int(0)<<16 | int(tmpBuf[1])<<8 | int(tmpBuf[2])
	if len(tmpBuf[3:]) < seq.ppsLen || seq.ppsLen <= 0 {
		return ppsDataError
	}

	pps = append(pps, StartCode4...)
	pps = append(pps, tmpBuf[3:]...)

	parser.specificInfo = append(parser.specificInfo, sps...)
	parser.specificInfo = append(parser.specificInfo, pps...)

	return nil
}

func (parser *Parser) isNaluHeader(src []byte) bool {
	if len(src) < naluBytesLen {
		return false
	}
	return src[0] == 0x00 &&
		src[1] == 0x00 &&
		src[2] == 0x00 &&
		src[3] == 0x01
}

func (parser *Parser) naluSize(src []byte) (int, error) {
	if len(src) < naluBytesLen {
		return 0, errors.New("nalusizedata invalid")
	}
	buf := src[:naluBytesLen]
	size := int(0)
	for i := 0; i < len(buf); i++ {
		size = size<<8 + int(buf[i])
	}
	return size, nil
}

func (parser *Parser) getAnnexbH264(src []byte, w io.Writer) error {
	dataSize := len(src)
	if dataSize < naluBytesLen {
		return videoDataInvalid
	}
	parser.pps.Reset()
	_, err := w.Write(naluAud)
	if err != nil {
		return err
	}

	index := 0
	nalLen := 0
	hasSpsPps := false
	hasWriteSpsPps := false

	for dataSize > 0 {
		nalLen, err = parser.naluSize(src[index:])
		if err != nil {
			return dataSizeNotMatch
		}
		index += naluBytesLen
		dataSize -= naluBytesLen
		if dataSize >= nalLen && len(src[index:]) >= nalLen && nalLen > 0 {
			nalType := src[index] & 0x1f
			switch nalType {
			case nalu_type_aud:
			case nalu_type_idr:
				if !hasWriteSpsPps {
					hasWriteSpsPps = true
					if !hasSpsPps {
						if _, err := w.Write(parser.specificInfo); err != nil {
							return err
						}
					} else {
						if _, err := w.Write(parser.pps.Bytes()); err != nil {
							return err
						}
					}
				}
				fallthrough
			case nalu_type_slice:
				fallthrough
			case nalu_type_sei:
				_, err := w.Write(StartCode4)
				if err != nil {
					return err
				}
				_, err = w.Write(src[index : index+nalLen])
				if err != nil {
					return err
				}
			case nalu_type_sps:
				fallthrough
			case nalu_type_pps:
				hasSpsPps = true
				_, err := parser.pps.Write(StartCode4)
				if err != nil {
					return err
				}
				_, err = parser.pps.Write(src[index : index+nalLen])
				if err != nil {
					return err
				}
			}
			index += nalLen
			dataSize -= nalLen
		} else {
			return naluBodyLenError
		}
	}
	return nil
}

func (parser *Parser) Parse(b []byte, isSeq bool, w io.Writer) (err error) {
	switch isSeq {
	case true:
		err = parser.parseSpecificInfo(b)
	case false:
		// is annexb
		if parser.isNaluHeader(b) {
			_, err = w.Write(b)
		} else {
			err = parser.getAnnexbH264(b, w)
		}
	}
	return
}

//ParseNalus 把src的数据按h264分隔符解析出来
func ParseNalus(src []byte) (nalus [][]byte) {
	return ParseNalusN(src, -1)
}

//ParseNalusN 按照00 00 00 01 分割nalu单元 n == 0 返回nil < 0 全部分割 > 0 最多分割 n 部分
func ParseNalusN(src []byte, n int) (nalus [][]byte) {
	nalus = make([][]byte, 0)
	if len(src) < naluBytesLen {
		fmt.Printf("invalid nalu len:%d\n", len(src))
		return nalus
	}

	if n == 0 {
		return nalus
	}

	pre := 0
	index := 0
	for {
		//先判断是否n已经为1了，如果是1，就不要继续分割，直接break
		//如果n > 1 或 < 0,继续分割
		if n == 1 {
			index = len(src)
			break
		}

		bfind := false
		startCodeLength := 0
		if bytes.Compare(src[index:index+3], StartCode3) == 0 {
			startCodeLength = 3
			bfind = true
		} else if bytes.Compare(src[index:index+4], StartCode4) == 0 {
			startCodeLength = 4
			bfind = true
		}

		if bfind {
			if index > pre {
				nalu := make([]byte, 0)
				nalu = append(nalu, src[pre:index]...)
				nalus = append(nalus, nalu)
				if n > 0 {
					n--
				}
			}
			index += startCodeLength
			pre = index
			continue
		}

		index++
		if index+4 > len(src) {
			break
		}
	}

	if index > pre && pre < len(src) {
		nalu := make([]byte, 0)
		nalu = append(nalu, src[pre:]...)
		nalus = append(nalus, nalu)
	}
	return
}

//AVCDecoderConfigurationRecord 生成h264的sequence header
func AVCDecoderConfigurationRecord(sps, pps []byte) []byte {
	//sps pps 去掉start code，否则可能会有问题 sps pps的start code是4位的
	if bytes.Compare(sps, StartCode4) == 0 {
		sps = sps[4:]
	}
	if bytes.Compare(pps, StartCode4) == 0 {
		pps = pps[4:]
	}
	sHeader := sequenceHeader{
		configVersion:        1,    //固定为1
		avcProfileIndication: 66,   //标清电视
		profileCompatility:   0,    //todo
		avcLevelIndication:   40,   //todo
		reserved1:            0x3f, // 0011 1111
		naluLen:              3,    //todo
		reserved2:            0x07, //0000 0111
		spsNum:               1,    //一般都只有一个
		ppsNum:               1,    //一般都只有一个
		spsLen:               len(sps),
		ppsLen:               len(pps),
	}

	index := 0
	buffer := make([]byte, 11+len(sps)+len(pps))
	buffer[index] = byte(sHeader.configVersion)
	index++ //0
	buffer[index] = byte(sHeader.avcProfileIndication)
	index++ //1
	buffer[index] = byte(sHeader.profileCompatility)
	index++ //2
	buffer[index] = byte(sHeader.avcLevelIndication)
	index++ //3
	//4
	buffer[index] = sHeader.reserved1 << 2
	buffer[index] |= sHeader.naluLen & 0x03
	index++
	//5
	buffer[index] = sHeader.reserved2 << 5
	buffer[index] |= sHeader.spsNum & 0x1F
	index++

	//sps length
	binary.BigEndian.PutUint16(buffer[6:], uint16(sHeader.spsLen))
	index += 2
	copy(buffer[index:], sps)
	index += len(sps)

	//pps number
	buffer[index] = 1
	index++
	binary.BigEndian.PutUint16(buffer[index:], uint16(sHeader.ppsLen))
	index += 2
	copy(buffer[index:], pps)
	index += len(pps)
	return buffer
}
