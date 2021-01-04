// package hls

// import (
// 	"bytes"
// 	"errors"
// 	"fmt"
// 	"time"

// 	"github.com/fabo871218/srtmp/av"
// 	"github.com/fabo871218/srtmp/container/flv"
// 	"github.com/fabo871218/srtmp/container/ts"
// 	parser "github.com/fabo871218/srtmp/media"
// )

// const (
// 	videoHZ      = 90000
// 	aacSampleLen = 1024
// 	maxQueueNum  = 512

// 	h264_default_hz uint64 = 90
// )

// type Source struct {
// 	av.RWBaser
// 	seq         int
// 	streamInfo  av.StreamInfo
// 	bwriter     *bytes.Buffer
// 	btswriter   *bytes.Buffer
// 	demuxer     *flv.Demuxer
// 	muxer       *ts.Muxer
// 	pts, dts    uint64
// 	stat        *status
// 	align       *align
// 	cache       *audioCache
// 	tsCache     *TSCacheItem
// 	tsparser    *parser.CodecParser
// 	closed      bool
// 	packetQueue chan *av.Packet
// }

// func NewSource(streamInfo av.StreamInfo) *Source {
// 	streamInfo.Inter = true
// 	s := &Source{
// 		streamInfo:  streamInfo,
// 		align:       &align{},
// 		stat:        newStatus(),
// 		RWBaser:     av.NewRWBaser(time.Second * 10),
// 		cache:       newAudioCache(),
// 		demuxer:     flv.NewDemuxer(),
// 		muxer:       ts.NewMuxer(),
// 		tsCache:     NewTSCacheItem(streamInfo.Key),
// 		tsparser:    parser.NewCodecParser(),
// 		bwriter:     bytes.NewBuffer(make([]byte, 100*1024)),
// 		packetQueue: make(chan *av.Packet, maxQueueNum),
// 	}
// 	go func() {
// 		err := s.SendPacket()
// 		if err != nil {
// 			fmt.Printf("send packet error: %v\n", err)
// 			s.closed = true
// 		}
// 	}()
// 	return s
// }

// func (source *Source) GetCacheInc() *TSCacheItem {
// 	return source.tsCache
// }

// func (source *Source) DropPacket(pktQue chan *av.Packet, streamInfo av.StreamInfo) {
// 	fmt.Printf("[%v] packet queue max!!!\n", streamInfo)
// 	for i := 0; i < maxQueueNum-84; i++ {
// 		tmpPkt, ok := <-pktQue
// 		// try to don't drop audio
// 		if !ok {
// 			continue
// 		}

// 		switch tmpPkt.PacketType {
// 		case av.PacketTypeAudio:
// 			if len(pktQue) > maxQueueNum-2 {
// 				<-pktQue
// 			} else {
// 				pktQue <- tmpPkt
// 			}
// 		case av.PacketTypeVideo:
// 			videoPkt, ok := tmpPkt.Header.(av.VideoPacketHeader)
// 			// dont't drop sps config and dont't drop key frame
// 			if ok && videoPkt.FrameType == av.FRAME_KEY {
// 				pktQue <- tmpPkt
// 			}
// 			if len(pktQue) > maxQueueNum-10 {
// 				<-pktQue
// 			}
// 		default:
// 		}
// 	}
// 	fmt.Printf("packet queue len:%d\n", len(pktQue))
// }

// func (source *Source) Write(p *av.Packet) (err error) {
// 	err = nil
// 	if source.closed {
// 		err = errors.New("hls source closed")
// 		return
// 	}
// 	source.SetPreTime()
// 	defer func() {
// 		if e := recover(); e != nil {
// 			errString := fmt.Sprintf("hls source has already been closed:%v", e)
// 			err = errors.New(errString)
// 		}
// 	}()
// 	if len(source.packetQueue) >= maxQueueNum-24 {
// 		source.DropPacket(source.packetQueue, source.streamInfo)
// 	} else {
// 		if !source.closed {
// 			source.packetQueue <- p
// 		}
// 	}
// 	return
// }

// func (source *Source) SendPacket() error {
// 	defer func() {
// 		fmt.Printf("[%v] hls sender stop\n", source.streamInfo)
// 		if r := recover(); r != nil {
// 			fmt.Printf("hls SendPacket panic: %v\n", r)
// 		}
// 	}()

// 	fmt.Printf("[%v] hls sender start\n", source.streamInfo)
// 	for {
// 		if source.closed {
// 			return errors.New("closed")
// 		}

// 		p, ok := <-source.packetQueue
// 		if ok {
// 			if p.PacketType == av.PacketTypeMetadata {
// 				continue
// 			}

// 			err := source.demuxer.Demux(p)
// 			if err == flv.ErrAvcEndSEQ {
// 				fmt.Printf("flv.ErrAvcEndSEQ\n")
// 				continue
// 			} else {
// 				if err != nil {
// 					return fmt.Errorf("demuxer.Demux failed, %v", err)
// 				}
// 			}
// 			compositionTime, isSeq, err := source.parse(p)
// 			if err != nil {
// 				fmt.Printf("source.parse failed, %v\n", err)
// 			}
// 			if err != nil || isSeq {
// 				continue
// 			}
// 			if source.btswriter != nil {
// 				isVideo := p.PacketType == av.PacketTypeVideo
// 				source.stat.update(isVideo, p.TimeStamp)
// 				source.calcPtsDts(isVideo, p.TimeStamp, uint32(compositionTime))
// 				source.tsMux(p)
// 			}
// 		} else {
// 			return errors.New("closed")
// 		}
// 	}
// }

// func (source *Source) StreamInfo() (ret av.StreamInfo) {
// 	return source.streamInfo
// }

// func (source *Source) cleanup() {
// 	close(source.packetQueue)
// 	source.bwriter = nil
// 	source.btswriter = nil
// 	source.cache = nil
// 	source.tsCache = nil
// }

// func (source *Source) Close() {
// 	fmt.Printf("hls source closed: %v\n", source.streamInfo)
// 	if !source.closed {
// 		source.cleanup()
// 	}
// 	source.closed = true
// }

// func (source *Source) cut() {
// 	newf := true
// 	if source.btswriter == nil {
// 		source.btswriter = bytes.NewBuffer(nil)
// 	} else if source.btswriter != nil && source.stat.durationMs() >= duration {
// 		source.flushAudio()

// 		source.seq++
// 		filename := fmt.Sprintf("/%s/%d.ts", source.streamInfo.Key, time.Now().Unix())
// 		item := NewTSItem(filename, int(source.stat.durationMs()), source.seq, source.btswriter.Bytes())
// 		source.tsCache.SetItem(filename, item)

// 		source.btswriter.Reset()
// 		source.stat.resetAndNew()
// 	} else {
// 		newf = false
// 	}
// 	if newf {
// 		source.btswriter.Write(source.muxer.PAT())
// 		source.btswriter.Write(source.muxer.PMT(av.SOUND_AAC, true))
// 	}
// }

// func (source *Source) parse(p *av.Packet) (int32, bool, error) {
// 	var compositionTime int32
// 	var ah av.AudioPacketHeader
// 	var vh av.VideoPacketHeader
// 	switch p.PacketType {
// 	case av.PacketTypeVideo:
// 		vh = p.Header.(av.VideoPacketHeader)
// 		if vh.CodecID != av.VIDEO_H264 {
// 			return compositionTime, false, ErrNoSupportVideoCodec
// 		}
// 		compositionTime = vh.CompositionTime
// 		if vh.FrameType == av.FRAME_KEY && vh.AVCPacketType == av.AVC_SEQHDR {
// 			return compositionTime, true, source.tsparser.Parse(p, source.bwriter)
// 		}
// 	case av.PacketTypeAudio:
// 		ah = p.Header.(av.AudioPacketHeader)
// 		if ah.SoundFormat != av.SOUND_AAC {
// 			return compositionTime, false, ErrNoSupportAudioCodec
// 		}
// 		if ah.AACPacketType == av.AAC_SEQHDR {
// 			return compositionTime, true, source.tsparser.Parse(p, source.bwriter)
// 		}
// 	}

// 	source.bwriter.Reset()
// 	if err := source.tsparser.Parse(p, source.bwriter); err != nil {
// 		return compositionTime, false, err
// 	}
// 	p.Data = source.bwriter.Bytes()

// 	if p.PacketType == av.PacketTypeVideo && vh.FrameType == av.FRAME_KEY {
// 		source.cut()
// 	}
// 	return compositionTime, false, nil
// }

// func (source *Source) calcPtsDts(isVideo bool, ts, compositionTs uint32) {
// 	source.dts = uint64(ts) * h264_default_hz
// 	if isVideo {
// 		source.pts = source.dts + uint64(compositionTs)*h264_default_hz
// 	} else {
// 		sampleRate, _ := source.tsparser.SampleRate()
// 		source.align.align(&source.dts, uint32(videoHZ*aacSampleLen/sampleRate))
// 		source.pts = source.dts
// 	}
// }
// func (source *Source) flushAudio() error {
// 	return source.muxAudio(1)
// }

// func (source *Source) muxAudio(limit byte) error {
// 	if source.cache.CacheNum() < limit {
// 		return nil
// 	}
// 	var p av.Packet
// 	_, pts, buf := source.cache.GetFrame()
// 	p.Data = buf
// 	p.TimeStamp = uint32(pts / h264_default_hz)
// 	return source.muxer.Mux(&p, source.btswriter)
// }

// func (source *Source) tsMux(p *av.Packet) error {
// 	if p.PacketType == av.PacketTypeVideo {
// 		return source.muxer.Mux(p, source.btswriter)
// 	} else {
// 		source.cache.Cache(p.Data, source.pts)
// 		return source.muxAudio(cache_max_frames)
// 	}
// }
