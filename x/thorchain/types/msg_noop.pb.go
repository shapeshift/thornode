// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: thorchain/v1/x/thorchain/types/msg_noop.proto

package types

import (
	fmt "fmt"
	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type MsgNoOp struct {
	ObservedTx ObservedTx                                    `protobuf:"bytes,1,opt,name=observed_tx,json=observedTx,proto3" json:"observed_tx"`
	Signer     github_com_cosmos_cosmos_sdk_types.AccAddress `protobuf:"bytes,2,opt,name=signer,proto3,casttype=github.com/cosmos/cosmos-sdk/types.AccAddress" json:"signer,omitempty"`
}

func (m *MsgNoOp) Reset()         { *m = MsgNoOp{} }
func (m *MsgNoOp) String() string { return proto.CompactTextString(m) }
func (*MsgNoOp) ProtoMessage()    {}
func (*MsgNoOp) Descriptor() ([]byte, []int) {
	return fileDescriptor_38ffd694543069d3, []int{0}
}
func (m *MsgNoOp) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MsgNoOp) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MsgNoOp.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MsgNoOp) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MsgNoOp.Merge(m, src)
}
func (m *MsgNoOp) XXX_Size() int {
	return m.Size()
}
func (m *MsgNoOp) XXX_DiscardUnknown() {
	xxx_messageInfo_MsgNoOp.DiscardUnknown(m)
}

var xxx_messageInfo_MsgNoOp proto.InternalMessageInfo

func (m *MsgNoOp) GetObservedTx() ObservedTx {
	if m != nil {
		return m.ObservedTx
	}
	return ObservedTx{}
}

func (m *MsgNoOp) GetSigner() github_com_cosmos_cosmos_sdk_types.AccAddress {
	if m != nil {
		return m.Signer
	}
	return nil
}

func init() {
	proto.RegisterType((*MsgNoOp)(nil), "types.MsgNoOp")
}

func init() {
	proto.RegisterFile("thorchain/v1/x/thorchain/types/msg_noop.proto", fileDescriptor_38ffd694543069d3)
}

var fileDescriptor_38ffd694543069d3 = []byte{
	// 260 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xd2, 0x2d, 0xc9, 0xc8, 0x2f,
	0x4a, 0xce, 0x48, 0xcc, 0xcc, 0xd3, 0x2f, 0x33, 0xd4, 0xaf, 0xd0, 0x47, 0x70, 0x4b, 0x2a, 0x0b,
	0x52, 0x8b, 0xf5, 0x73, 0x8b, 0xd3, 0xe3, 0xf3, 0xf2, 0xf3, 0x0b, 0xf4, 0x0a, 0x8a, 0xf2, 0x4b,
	0xf2, 0x85, 0x58, 0xc1, 0xa2, 0x52, 0xa6, 0x04, 0x74, 0x81, 0xc8, 0xf8, 0xfc, 0xa4, 0xe2, 0xd4,
	0xa2, 0xb2, 0xd4, 0x94, 0xf8, 0x92, 0x0a, 0x88, 0x6e, 0x29, 0x91, 0xf4, 0xfc, 0xf4, 0x7c, 0x30,
	0x53, 0x1f, 0xc4, 0x82, 0x88, 0x2a, 0xf5, 0x31, 0x72, 0xb1, 0xfb, 0x16, 0xa7, 0xfb, 0xe5, 0xfb,
	0x17, 0x08, 0x59, 0x70, 0x71, 0x23, 0x69, 0x93, 0x60, 0x54, 0x60, 0xd4, 0xe0, 0x36, 0x12, 0xd4,
	0x03, 0x9b, 0xaa, 0xe7, 0x0f, 0x95, 0x09, 0xa9, 0x70, 0x62, 0x39, 0x71, 0x4f, 0x9e, 0x21, 0x88,
	0x2b, 0x1f, 0x2e, 0x22, 0xe4, 0xc9, 0xc5, 0x56, 0x9c, 0x99, 0x9e, 0x97, 0x5a, 0x24, 0xc1, 0xa4,
	0xc0, 0xa8, 0xc1, 0xe3, 0x64, 0xf8, 0xeb, 0x9e, 0xbc, 0x6e, 0x7a, 0x66, 0x49, 0x46, 0x69, 0x92,
	0x5e, 0x72, 0x7e, 0xae, 0x7e, 0x72, 0x7e, 0x71, 0x6e, 0x7e, 0x31, 0x94, 0xd2, 0x2d, 0x4e, 0xc9,
	0x86, 0x38, 0x55, 0xcf, 0x31, 0x39, 0xd9, 0x31, 0x25, 0xa5, 0x28, 0xb5, 0xb8, 0x38, 0x08, 0x6a,
	0x80, 0x93, 0xe7, 0x89, 0x47, 0x72, 0x8c, 0x17, 0x1e, 0xc9, 0x31, 0x3e, 0x78, 0x24, 0xc7, 0x38,
	0xe1, 0xb1, 0x1c, 0xc3, 0x85, 0xc7, 0x72, 0x0c, 0x37, 0x1e, 0xcb, 0x31, 0x44, 0xe9, 0xa7, 0x67,
	0x96, 0xe4, 0x24, 0x42, 0x0c, 0x44, 0xf2, 0x73, 0x46, 0x7e, 0x51, 0x5e, 0x7e, 0x4a, 0x2a, 0x66,
	0x40, 0x24, 0xb1, 0x81, 0xbd, 0x68, 0x0c, 0x08, 0x00, 0x00, 0xff, 0xff, 0xea, 0x60, 0xac, 0x20,
	0x67, 0x01, 0x00, 0x00,
}

func (m *MsgNoOp) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MsgNoOp) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgNoOp) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Signer) > 0 {
		i -= len(m.Signer)
		copy(dAtA[i:], m.Signer)
		i = encodeVarintMsgNoop(dAtA, i, uint64(len(m.Signer)))
		i--
		dAtA[i] = 0x12
	}
	{
		size, err := m.ObservedTx.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintMsgNoop(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0xa
	return len(dAtA) - i, nil
}

func encodeVarintMsgNoop(dAtA []byte, offset int, v uint64) int {
	offset -= sovMsgNoop(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *MsgNoOp) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = m.ObservedTx.Size()
	n += 1 + l + sovMsgNoop(uint64(l))
	l = len(m.Signer)
	if l > 0 {
		n += 1 + l + sovMsgNoop(uint64(l))
	}
	return n
}

func sovMsgNoop(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozMsgNoop(x uint64) (n int) {
	return sovMsgNoop(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *MsgNoOp) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMsgNoop
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MsgNoOp: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MsgNoOp: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ObservedTx", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgNoop
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMsgNoop
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthMsgNoop
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.ObservedTx.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Signer", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgNoop
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthMsgNoop
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthMsgNoop
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Signer = append(m.Signer[:0], dAtA[iNdEx:postIndex]...)
			if m.Signer == nil {
				m.Signer = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMsgNoop(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMsgNoop
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthMsgNoop
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipMsgNoop(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowMsgNoop
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMsgNoop
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMsgNoop
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthMsgNoop
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupMsgNoop
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthMsgNoop
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthMsgNoop        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowMsgNoop          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupMsgNoop = fmt.Errorf("proto: unexpected end of group")
)
