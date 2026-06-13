package obd

import (
	"encoding/binary"
	"fmt"
)

// Minimal SDP (Service Discovery Protocol) support for resolving the RFCOMM
// channel of a device's Serial Port Profile. This is just enough of SDP to build
// one ServiceSearchAttributeRequest and pull the channel out of the response —
// it is not a general SDP implementation.

const (
	sdpServiceSearchAttrRequest  = 0x06
	sdpServiceSearchAttrResponse = 0x07

	// uuidSerialPort is the Serial Port Profile service class (SPP).
	uuidSerialPort = 0x1101
	// uuidRFCOMM is the RFCOMM protocol UUID; the channel follows it in a
	// protocol descriptor.
	uuidRFCOMM = 0x0003
	// attrProtocolDescriptorList is the SDP attribute holding the protocol stack.
	attrProtocolDescriptorList = 0x0004

	// Data element descriptor bytes used below.
	deUUID16 = 0x19 // UUID, 2 bytes
	deUint8  = 0x08 // unsigned int, 1 byte
	deUint16 = 0x09 // unsigned int, 2 bytes
	deSeq8   = 0x35 // sequence, 8-bit length prefix
)

// buildServiceSearchAttributeRequest builds an SDP request that searches for the
// Serial Port service and asks for its ProtocolDescriptorList attribute.
func buildServiceSearchAttributeRequest(transactionID uint16) []byte {
	// ServiceSearchPattern: sequence containing the SerialPort UUID.
	searchPattern := []byte{deSeq8, 0x03, deUUID16, byte(uuidSerialPort >> 8), byte(uuidSerialPort & 0xFF)}

	// AttributeIDList: sequence containing the ProtocolDescriptorList attribute.
	attrList := []byte{deSeq8, 0x03, deUint16, byte(attrProtocolDescriptorList >> 8), byte(attrProtocolDescriptorList)}

	var params []byte
	params = append(params, searchPattern...)
	params = append(params, 0xFF, 0xFF) // MaximumAttributeByteCount
	params = append(params, attrList...)
	params = append(params, 0x00) // ContinuationState: none

	pdu := make([]byte, 5+len(params))
	pdu[0] = sdpServiceSearchAttrRequest
	binary.BigEndian.PutUint16(pdu[1:3], transactionID)
	binary.BigEndian.PutUint16(pdu[3:5], uint16(len(params)))
	copy(pdu[5:], params)
	return pdu
}

// parseRFCOMMChannel extracts the RFCOMM channel from an SDP
// ServiceSearchAttributeResponse. It scans the attribute data for the RFCOMM
// protocol UUID (0x0003) followed by an unsigned 1-byte channel, which is how a
// standard SPP ProtocolDescriptorList encodes it.
func parseRFCOMMChannel(resp []byte) (uint8, error) {
	if len(resp) < 5 || resp[0] != sdpServiceSearchAttrResponse {
		return 0, fmt.Errorf("not an SDP attribute response")
	}
	// Walk the payload looking for: UUID16(0x0003) then Uint8(channel).
	body := resp[5:]
	for i := 0; i+4 < len(body); i++ {
		if body[i] == deUUID16 &&
			body[i+1] == byte(uuidRFCOMM>>8) && body[i+2] == byte(uuidRFCOMM) &&
			body[i+3] == deUint8 {
			return body[i+4], nil
		}
	}
	return 0, fmt.Errorf("no RFCOMM channel found in SDP response")
}
