// Copyright 2012 Google, Inc. All rights reserved.
// Copyright 2009-2011 Andreas Krennmair. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

package layers

import (
	"code.google.com/p/gopacket"
	"encoding/binary"
	"errors"
	"fmt"
)

// TCP is the layer for TCP headers.
type TCP struct {
	baseLayer
	SrcPort, DstPort                           TCPPort
	Seq                                        uint32
	Ack                                        uint32
	DataOffset                                 uint8
	FIN, SYN, RST, PSH, ACK, URG, ECE, CWR, NS bool
	Window                                     uint16
	Checksum                                   uint16
	Urgent                                     uint16
	sPort, dPort                               []byte
	Options                                    []TCPOption
	Padding                                    []byte
	opts                                       [4]TCPOption
}

type TCPOption struct {
	OptionType   uint8
	OptionLength uint8
	OptionData   []byte
}

func (t TCPOption) String() string {
	switch t.OptionType {
	case 1:
		return "NOP"
	case 8:
		if len(t.OptionData) == 8 {
			return fmt.Sprintf("TSOPT:%v/%v",
				binary.BigEndian.Uint32(t.OptionData[:4]),
				binary.BigEndian.Uint32(t.OptionData[4:8]))
		}
	}
	return fmt.Sprintf("TCPOption(%v:%v)", t.OptionType, t.OptionData)
}

// LayerType returns gopacket.LayerTypeTCP
func (t *TCP) LayerType() gopacket.LayerType { return LayerTypeTCP }

func (tcp *TCP) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) ([]byte, error) {
	tcp.SrcPort = TCPPort(binary.BigEndian.Uint16(data[0:2]))
	tcp.sPort = data[0:2]
	tcp.DstPort = TCPPort(binary.BigEndian.Uint16(data[2:4]))
	tcp.dPort = data[2:4]
	tcp.Seq = binary.BigEndian.Uint32(data[4:8])
	tcp.Ack = binary.BigEndian.Uint32(data[8:12])
	tcp.DataOffset = (data[12] & 0xF0) >> 4
	tcp.FIN = data[13]&0x01 != 0
	tcp.SYN = data[13]&0x02 != 0
	tcp.RST = data[13]&0x04 != 0
	tcp.PSH = data[13]&0x08 != 0
	tcp.ACK = data[13]&0x10 != 0
	tcp.URG = data[13]&0x20 != 0
	tcp.ECE = data[13]&0x40 != 0
	tcp.CWR = data[13]&0x80 != 0
	tcp.NS = data[12]&0x01 != 0
	tcp.Window = binary.BigEndian.Uint16(data[14:16])
	tcp.Checksum = binary.BigEndian.Uint16(data[16:18])
	tcp.Urgent = binary.BigEndian.Uint16(data[18:20])
	tcp.Options = nil
	if tcp.DataOffset < 5 {
		return nil, fmt.Errorf("Invalid TCP data offset %d < 5", tcp.DataOffset)
	}
	dataStart := int(tcp.DataOffset) * 4
	if dataStart > len(data) {
		df.SetTruncated()
		tcp.payload = nil
		tcp.contents = data
		return nil, errors.New("TCP data offset greater than packet length")
	}
	tcp.contents = data[:dataStart]
	tcp.payload = data[dataStart:]
	// From here on, data points just to the header options.
	data = data[20:dataStart]
	for len(data) > 0 {
		if tcp.Options == nil {
			// Pre-allocate to avoid allocating a slice.
			tcp.Options = tcp.opts[:0]
		}
		tcp.Options = append(tcp.Options, TCPOption{OptionType: data[0]})
		opt := &tcp.Options[len(tcp.Options)-1]
		switch opt.OptionType {
		case 0: // End of options
			opt.OptionLength = 1
			tcp.Padding = data[1:]
			break
		case 1: // 1 byte padding
			opt.OptionLength = 1
		default:
			opt.OptionLength = data[1]
			if opt.OptionLength < 2 {
				return nil, fmt.Errorf("Invalid TCP option length %d < 2", opt.OptionLength)
			} else if int(opt.OptionLength) > len(data) {
				return nil, fmt.Errorf("Invalid TCP option length %d exceeds remaining %d bytes", opt.OptionLength, len(data))
			}
			opt.OptionData = data[2:opt.OptionLength]
		}
		data = data[opt.OptionLength:]
	}
	return tcp.payload, nil
}

func decodeTCP(data []byte, p gopacket.PacketBuilder) (err error) {
	tcp := &TCP{}
	_, err = tcp.DecodeFromBytes(data, p)
	p.AddLayer(tcp)
	p.SetTransportLayer(tcp)
	if err == nil {
		err = p.NextDecoder(gopacket.LayerTypePayload)
	}
	return err
}

func (t *TCP) TransportFlow() gopacket.Flow {
	return gopacket.NewFlow(EndpointTCPPort, t.sPort, t.dPort)
}