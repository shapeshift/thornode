// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: thorchain/v1/x/thorchain/types/type_node_account.proto

package types

import (
	fmt "fmt"
	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	common "gitlab.com/thorchain/thornode/common"
	gitlab_com_thorchain_thornode_common "gitlab.com/thorchain/thornode/common"
	io "io"
	math "math"
	math_bits "math/bits"
	reflect "reflect"
	strings "strings"
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

type NodeStatus int32

const (
	NodeStatus_Unknown     NodeStatus = 0
	NodeStatus_Whitelisted NodeStatus = 1
	NodeStatus_Standby     NodeStatus = 2
	NodeStatus_Ready       NodeStatus = 3
	NodeStatus_Active      NodeStatus = 4
	NodeStatus_Disabled    NodeStatus = 5
)

var NodeStatus_name = map[int32]string{
	0: "Unknown",
	1: "Whitelisted",
	2: "Standby",
	3: "Ready",
	4: "Active",
	5: "Disabled",
}

var NodeStatus_value = map[string]int32{
	"Unknown":     0,
	"Whitelisted": 1,
	"Standby":     2,
	"Ready":       3,
	"Active":      4,
	"Disabled":    5,
}

func (x NodeStatus) String() string {
	return proto.EnumName(NodeStatus_name, int32(x))
}

func (NodeStatus) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_553b0a276f7a08b4, []int{0}
}

type NodeType int32

const (
	NodeType_TypeValidator NodeType = 0
	NodeType_TypeVault     NodeType = 1
	NodeType_TypeUnknown   NodeType = 2
)

var NodeType_name = map[int32]string{
	0: "TypeValidator",
	1: "TypeVault",
	2: "TypeUnknown",
}

var NodeType_value = map[string]int32{
	"TypeValidator": 0,
	"TypeVault":     1,
	"TypeUnknown":   2,
}

func (x NodeType) String() string {
	return proto.EnumName(NodeType_name, int32(x))
}

func (NodeType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_553b0a276f7a08b4, []int{1}
}

type NodeAccount struct {
	NodeAddress         github_com_cosmos_cosmos_sdk_types.AccAddress `protobuf:"bytes,1,opt,name=node_address,json=nodeAddress,proto3,casttype=github.com/cosmos/cosmos-sdk/types.AccAddress" json:"node_address,omitempty"`
	Status              NodeStatus                                    `protobuf:"varint,2,opt,name=status,proto3,enum=types.NodeStatus" json:"status,omitempty"`
	PubKeySet           common.PubKeySet                              `protobuf:"bytes,3,opt,name=pub_key_set,json=pubKeySet,proto3" json:"pub_key_set"`
	ValidatorConsPubKey string                                        `protobuf:"bytes,4,opt,name=validator_cons_pub_key,json=validatorConsPubKey,proto3" json:"validator_cons_pub_key,omitempty"`
	Bond                github_com_cosmos_cosmos_sdk_types.Uint       `protobuf:"bytes,5,opt,name=bond,proto3,customtype=github.com/cosmos/cosmos-sdk/types.Uint" json:"bond"`
	ActiveBlockHeight   int64                                         `protobuf:"varint,6,opt,name=active_block_height,json=activeBlockHeight,proto3" json:"active_block_height,omitempty"`
	BondAddress         gitlab_com_thorchain_thornode_common.Address  `protobuf:"bytes,7,opt,name=bond_address,json=bondAddress,proto3,casttype=gitlab.com/thorchain/thornode/common.Address" json:"bond_address,omitempty"`
	StatusSince         int64                                         `protobuf:"varint,8,opt,name=status_since,json=statusSince,proto3" json:"status_since,omitempty"`
	SignerMembership    []string                                      `protobuf:"bytes,9,rep,name=signer_membership,json=signerMembership,proto3" json:"signer_membership,omitempty"`
	RequestedToLeave    bool                                          `protobuf:"varint,10,opt,name=requested_to_leave,json=requestedToLeave,proto3" json:"requested_to_leave,omitempty"`
	ForcedToLeave       bool                                          `protobuf:"varint,11,opt,name=forced_to_leave,json=forcedToLeave,proto3" json:"forced_to_leave,omitempty"`
	LeaveScore          uint64                                        `protobuf:"varint,12,opt,name=leave_score,json=leaveScore,proto3" json:"leave_score,omitempty"`
	IPAddress           string                                        `protobuf:"bytes,13,opt,name=ip_address,json=ipAddress,proto3" json:"ip_address,omitempty"`
	Version             string                                        `protobuf:"bytes,14,opt,name=version,proto3" json:"version,omitempty"`
	Type                NodeType                                      `protobuf:"varint,15,opt,name=type,proto3,enum=types.NodeType" json:"type,omitempty"`
}

func (m *NodeAccount) Reset()      { *m = NodeAccount{} }
func (*NodeAccount) ProtoMessage() {}
func (*NodeAccount) Descriptor() ([]byte, []int) {
	return fileDescriptor_553b0a276f7a08b4, []int{0}
}
func (m *NodeAccount) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *NodeAccount) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_NodeAccount.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *NodeAccount) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeAccount.Merge(m, src)
}
func (m *NodeAccount) XXX_Size() int {
	return m.Size()
}
func (m *NodeAccount) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeAccount.DiscardUnknown(m)
}

var xxx_messageInfo_NodeAccount proto.InternalMessageInfo

type BondProvider struct {
	BondAddress github_com_cosmos_cosmos_sdk_types.AccAddress `protobuf:"bytes,1,opt,name=bond_address,json=bondAddress,proto3,casttype=github.com/cosmos/cosmos-sdk/types.AccAddress" json:"bond_address,omitempty"`
	Bond        github_com_cosmos_cosmos_sdk_types.Uint       `protobuf:"bytes,2,opt,name=bond,proto3,customtype=github.com/cosmos/cosmos-sdk/types.Uint" json:"bond"`
}

func (m *BondProvider) Reset()      { *m = BondProvider{} }
func (*BondProvider) ProtoMessage() {}
func (*BondProvider) Descriptor() ([]byte, []int) {
	return fileDescriptor_553b0a276f7a08b4, []int{1}
}
func (m *BondProvider) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *BondProvider) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_BondProvider.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *BondProvider) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BondProvider.Merge(m, src)
}
func (m *BondProvider) XXX_Size() int {
	return m.Size()
}
func (m *BondProvider) XXX_DiscardUnknown() {
	xxx_messageInfo_BondProvider.DiscardUnknown(m)
}

var xxx_messageInfo_BondProvider proto.InternalMessageInfo

type BondProviders struct {
	NodeAddress github_com_cosmos_cosmos_sdk_types.AccAddress `protobuf:"bytes,1,opt,name=node_address,json=nodeAddress,proto3,casttype=github.com/cosmos/cosmos-sdk/types.AccAddress" json:"node_address,omitempty"`
	Providers   []BondProvider                                `protobuf:"bytes,2,rep,name=providers,proto3" json:"providers"`
}

func (m *BondProviders) Reset()      { *m = BondProviders{} }
func (*BondProviders) ProtoMessage() {}
func (*BondProviders) Descriptor() ([]byte, []int) {
	return fileDescriptor_553b0a276f7a08b4, []int{2}
}
func (m *BondProviders) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *BondProviders) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_BondProviders.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *BondProviders) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BondProviders.Merge(m, src)
}
func (m *BondProviders) XXX_Size() int {
	return m.Size()
}
func (m *BondProviders) XXX_DiscardUnknown() {
	xxx_messageInfo_BondProviders.DiscardUnknown(m)
}

var xxx_messageInfo_BondProviders proto.InternalMessageInfo

func init() {
	proto.RegisterEnum("types.NodeStatus", NodeStatus_name, NodeStatus_value)
	proto.RegisterEnum("types.NodeType", NodeType_name, NodeType_value)
	proto.RegisterType((*NodeAccount)(nil), "types.NodeAccount")
	proto.RegisterType((*BondProvider)(nil), "types.BondProvider")
	proto.RegisterType((*BondProviders)(nil), "types.BondProviders")
}

func init() {
	proto.RegisterFile("thorchain/v1/x/thorchain/types/type_node_account.proto", fileDescriptor_553b0a276f7a08b4)
}

var fileDescriptor_553b0a276f7a08b4 = []byte{
	// 781 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xb4, 0x54, 0x4f, 0x6f, 0xe3, 0x44,
	0x14, 0xf7, 0x24, 0x69, 0x5a, 0x3f, 0x27, 0x5b, 0x67, 0x8a, 0x90, 0xd5, 0x83, 0x63, 0x8a, 0x04,
	0x66, 0xe9, 0x26, 0x6c, 0x57, 0x62, 0x25, 0x6e, 0x4d, 0x39, 0x80, 0xf8, 0xa3, 0xca, 0x69, 0x41,
	0xe2, 0x62, 0xf9, 0xcf, 0x10, 0x8f, 0x9a, 0xcc, 0x18, 0xcf, 0x24, 0x90, 0x5b, 0x3f, 0x02, 0x1f,
	0x62, 0x0f, 0xdc, 0xf8, 0x1a, 0x3d, 0xee, 0x71, 0x85, 0x50, 0xc4, 0xa6, 0xdf, 0x62, 0x4f, 0x68,
	0x66, 0xe2, 0x6c, 0x56, 0x48, 0x08, 0x81, 0xb8, 0x78, 0x3c, 0xbf, 0xdf, 0x9b, 0xf7, 0xde, 0xbc,
	0xf7, 0x7e, 0x03, 0x1f, 0xcb, 0x82, 0x57, 0x59, 0x91, 0x50, 0x36, 0x5c, 0x3c, 0x1e, 0xfe, 0x34,
	0x7c, 0xbd, 0x95, 0xcb, 0x92, 0x08, 0xfd, 0x8d, 0x19, 0xcf, 0x49, 0x9c, 0x64, 0x19, 0x9f, 0x33,
	0x39, 0x28, 0x2b, 0x2e, 0x39, 0xde, 0xd3, 0xf4, 0x71, 0xf0, 0xc6, 0xf1, 0x8c, 0xcf, 0x66, 0x9c,
	0x6d, 0x16, 0x63, 0x78, 0xfc, 0xd6, 0x84, 0x4f, 0xb8, 0xfe, 0x1d, 0xaa, 0x3f, 0x83, 0x9e, 0xdc,
	0xb6, 0xc1, 0xf9, 0x9a, 0xe7, 0xe4, 0xdc, 0x38, 0xc5, 0x57, 0xd0, 0x31, 0x41, 0xf2, 0xbc, 0x22,
	0x42, 0x78, 0x28, 0x40, 0x61, 0x67, 0xf4, 0xf8, 0xd5, 0xaa, 0xff, 0x68, 0x42, 0x65, 0x31, 0x4f,
	0x07, 0x19, 0x9f, 0x0d, 0x33, 0x2e, 0x66, 0x5c, 0x6c, 0x96, 0x47, 0x22, 0xbf, 0x31, 0x49, 0x0e,
	0xce, 0xb3, 0xec, 0xdc, 0x1c, 0x8c, 0x1c, 0xe5, 0x66, 0xb3, 0xc1, 0x1f, 0x40, 0x5b, 0xc8, 0x44,
	0xce, 0x85, 0xd7, 0x08, 0x50, 0xf8, 0xe0, 0xac, 0x37, 0x30, 0xf6, 0x2a, 0xf2, 0x58, 0x13, 0xd1,
	0xc6, 0x00, 0x3f, 0x05, 0xa7, 0x9c, 0xa7, 0xf1, 0x0d, 0x59, 0xc6, 0x82, 0x48, 0xaf, 0x19, 0xa0,
	0xd0, 0x39, 0xeb, 0x0d, 0x36, 0x57, 0xb9, 0x9c, 0xa7, 0x5f, 0x90, 0xe5, 0x98, 0xc8, 0x51, 0xeb,
	0x6e, 0xd5, 0xb7, 0x22, 0xbb, 0xac, 0x01, 0xfc, 0x04, 0xde, 0x5e, 0x24, 0x53, 0x9a, 0x27, 0x92,
	0x57, 0x71, 0xc6, 0x99, 0x88, 0x37, 0x7e, 0xbc, 0x56, 0x80, 0x42, 0x3b, 0x3a, 0xda, 0xb2, 0x17,
	0x9c, 0x09, 0xe3, 0x08, 0x5f, 0x40, 0x2b, 0xe5, 0x2c, 0xf7, 0xf6, 0x94, 0xc9, 0x68, 0xa8, 0x7c,
	0xfe, 0xb6, 0xea, 0xbf, 0xff, 0x0f, 0xae, 0x7a, 0x4d, 0x99, 0x8c, 0xf4, 0x61, 0x3c, 0x80, 0xa3,
	0x24, 0x93, 0x74, 0x41, 0xe2, 0x74, 0xca, 0xb3, 0x9b, 0xb8, 0x20, 0x74, 0x52, 0x48, 0xaf, 0x1d,
	0xa0, 0xb0, 0x19, 0xf5, 0x0c, 0x35, 0x52, 0xcc, 0x67, 0x9a, 0xc0, 0x63, 0xe8, 0xa8, 0x73, 0xdb,
	0x1a, 0xef, 0xeb, 0xe0, 0x1f, 0xbd, 0x5a, 0xf5, 0x4f, 0x27, 0x54, 0x4e, 0x13, 0x13, 0x78, 0x67,
	0x00, 0x0a, 0x5e, 0xa9, 0x6a, 0xd6, 0xfd, 0xdc, 0x96, 0x58, 0x79, 0xa9, 0x4b, 0xfc, 0x0e, 0x74,
	0x4c, 0x05, 0x63, 0x41, 0x59, 0x46, 0xbc, 0x03, 0x1d, 0xdd, 0x31, 0xd8, 0x58, 0x41, 0xf8, 0x43,
	0xe8, 0x09, 0x3a, 0x61, 0xa4, 0x8a, 0x67, 0x64, 0x96, 0x92, 0x4a, 0x14, 0xb4, 0xf4, 0xec, 0xa0,
	0x19, 0xda, 0x91, 0x6b, 0x88, 0xaf, 0xb6, 0x38, 0x3e, 0x05, 0x5c, 0x91, 0x1f, 0xe6, 0x44, 0x48,
	0x92, 0xc7, 0x92, 0xc7, 0x53, 0x92, 0x2c, 0x88, 0x07, 0x01, 0x0a, 0x0f, 0x22, 0x77, 0xcb, 0x5c,
	0xf1, 0x2f, 0x15, 0x8e, 0xdf, 0x83, 0xc3, 0xef, 0x79, 0x95, 0xed, 0x9a, 0x3a, 0xda, 0xb4, 0x6b,
	0xe0, 0xda, 0xae, 0x0f, 0x8e, 0x66, 0x63, 0x91, 0xf1, 0x8a, 0x78, 0x9d, 0x00, 0x85, 0xad, 0x08,
	0x34, 0x34, 0x56, 0x08, 0x3e, 0x05, 0xa0, 0xe5, 0xb6, 0x32, 0x5d, 0x5d, 0x99, 0xee, 0x7a, 0xd5,
	0xb7, 0x3f, 0xbf, 0xac, 0xaf, 0x6d, 0xd3, 0xb2, 0xbe, 0xb4, 0x07, 0xfb, 0x0b, 0x52, 0x09, 0xca,
	0x99, 0xf7, 0x40, 0x37, 0xb9, 0xde, 0xe2, 0x77, 0xa1, 0xa5, 0xfa, 0xe4, 0x1d, 0xea, 0x79, 0x3b,
	0xdc, 0x99, 0xb7, 0xab, 0x65, 0x49, 0x22, 0x4d, 0x7e, 0xd2, 0xba, 0xfd, 0x3d, 0xb0, 0x4e, 0x7e,
	0x45, 0xd0, 0x19, 0x71, 0x96, 0x5f, 0x56, 0x7c, 0x41, 0x73, 0x52, 0x29, 0x0d, 0xbc, 0xd1, 0x9f,
	0x7f, 0xaf, 0x81, 0xdd, 0x06, 0xd5, 0xa3, 0xd6, 0xf8, 0x0f, 0xa3, 0xa6, 0x33, 0x46, 0x27, 0xcf,
	0x10, 0x74, 0x77, 0x33, 0x16, 0xff, 0x93, 0x6c, 0x9f, 0x82, 0x5d, 0xd6, 0x21, 0xbc, 0x46, 0xd0,
	0x0c, 0x9d, 0xb3, 0xa3, 0x4d, 0x25, 0x77, 0xc3, 0x6f, 0xb5, 0x58, 0xdb, 0x9a, 0x34, 0x1f, 0xa6,
	0x00, 0xaf, 0x05, 0x8e, 0x1d, 0xd8, 0xbf, 0x66, 0x37, 0x8c, 0xff, 0xc8, 0x5c, 0x0b, 0x1f, 0x82,
	0xf3, 0x6d, 0x41, 0x25, 0x99, 0x52, 0x35, 0x45, 0x2e, 0x52, 0xec, 0x58, 0x26, 0x2c, 0x4f, 0x97,
	0x6e, 0x03, 0xdb, 0xb0, 0x17, 0x91, 0x24, 0x5f, 0xba, 0x4d, 0x0c, 0xd0, 0x3e, 0xd7, 0x02, 0x72,
	0x5b, 0xb8, 0x03, 0x07, 0x9f, 0x52, 0x91, 0xa4, 0x53, 0x92, 0xbb, 0x7b, 0xc7, 0xad, 0x5f, 0x9e,
	0xf9, 0xe8, 0xe1, 0x05, 0x1c, 0xd4, 0x4d, 0xc5, 0x3d, 0xe8, 0xaa, 0xf5, 0x9b, 0x5a, 0xe7, 0xae,
	0x85, 0xbb, 0x60, 0x1b, 0x68, 0x3e, 0x95, 0x2e, 0x52, 0x61, 0xd5, 0xb6, 0xce, 0xa3, 0x61, 0x9c,
	0x8c, 0xae, 0xef, 0x5e, 0xfa, 0xd6, 0x8b, 0x97, 0xbe, 0x75, 0xbb, 0xf6, 0xad, 0xbb, 0xb5, 0x8f,
	0x9e, 0xaf, 0x7d, 0xf4, 0xc7, 0xda, 0x47, 0x3f, 0xdf, 0xfb, 0xd6, 0xf3, 0x7b, 0xdf, 0x7a, 0x71,
	0xef, 0x5b, 0xdf, 0x0d, 0xff, 0x5e, 0x9c, 0x7f, 0x79, 0xb2, 0xd3, 0xb6, 0x7e, 0x62, 0x9f, 0xfc,
	0x19, 0x00, 0x00, 0xff, 0xff, 0xd4, 0xfa, 0x77, 0x82, 0xdb, 0x05, 0x00, 0x00,
}

func (m *NodeAccount) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *NodeAccount) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *NodeAccount) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Type != 0 {
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(m.Type))
		i--
		dAtA[i] = 0x78
	}
	if len(m.Version) > 0 {
		i -= len(m.Version)
		copy(dAtA[i:], m.Version)
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(len(m.Version)))
		i--
		dAtA[i] = 0x72
	}
	if len(m.IPAddress) > 0 {
		i -= len(m.IPAddress)
		copy(dAtA[i:], m.IPAddress)
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(len(m.IPAddress)))
		i--
		dAtA[i] = 0x6a
	}
	if m.LeaveScore != 0 {
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(m.LeaveScore))
		i--
		dAtA[i] = 0x60
	}
	if m.ForcedToLeave {
		i--
		if m.ForcedToLeave {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x58
	}
	if m.RequestedToLeave {
		i--
		if m.RequestedToLeave {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x50
	}
	if len(m.SignerMembership) > 0 {
		for iNdEx := len(m.SignerMembership) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.SignerMembership[iNdEx])
			copy(dAtA[i:], m.SignerMembership[iNdEx])
			i = encodeVarintTypeNodeAccount(dAtA, i, uint64(len(m.SignerMembership[iNdEx])))
			i--
			dAtA[i] = 0x4a
		}
	}
	if m.StatusSince != 0 {
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(m.StatusSince))
		i--
		dAtA[i] = 0x40
	}
	if len(m.BondAddress) > 0 {
		i -= len(m.BondAddress)
		copy(dAtA[i:], m.BondAddress)
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(len(m.BondAddress)))
		i--
		dAtA[i] = 0x3a
	}
	if m.ActiveBlockHeight != 0 {
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(m.ActiveBlockHeight))
		i--
		dAtA[i] = 0x30
	}
	{
		size := m.Bond.Size()
		i -= size
		if _, err := m.Bond.MarshalTo(dAtA[i:]); err != nil {
			return 0, err
		}
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x2a
	if len(m.ValidatorConsPubKey) > 0 {
		i -= len(m.ValidatorConsPubKey)
		copy(dAtA[i:], m.ValidatorConsPubKey)
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(len(m.ValidatorConsPubKey)))
		i--
		dAtA[i] = 0x22
	}
	{
		size, err := m.PubKeySet.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x1a
	if m.Status != 0 {
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(m.Status))
		i--
		dAtA[i] = 0x10
	}
	if len(m.NodeAddress) > 0 {
		i -= len(m.NodeAddress)
		copy(dAtA[i:], m.NodeAddress)
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(len(m.NodeAddress)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *BondProvider) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BondProvider) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *BondProvider) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	{
		size := m.Bond.Size()
		i -= size
		if _, err := m.Bond.MarshalTo(dAtA[i:]); err != nil {
			return 0, err
		}
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x12
	if len(m.BondAddress) > 0 {
		i -= len(m.BondAddress)
		copy(dAtA[i:], m.BondAddress)
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(len(m.BondAddress)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *BondProviders) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *BondProviders) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *BondProviders) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Providers) > 0 {
		for iNdEx := len(m.Providers) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Providers[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintTypeNodeAccount(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.NodeAddress) > 0 {
		i -= len(m.NodeAddress)
		copy(dAtA[i:], m.NodeAddress)
		i = encodeVarintTypeNodeAccount(dAtA, i, uint64(len(m.NodeAddress)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintTypeNodeAccount(dAtA []byte, offset int, v uint64) int {
	offset -= sovTypeNodeAccount(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *NodeAccount) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.NodeAddress)
	if l > 0 {
		n += 1 + l + sovTypeNodeAccount(uint64(l))
	}
	if m.Status != 0 {
		n += 1 + sovTypeNodeAccount(uint64(m.Status))
	}
	l = m.PubKeySet.Size()
	n += 1 + l + sovTypeNodeAccount(uint64(l))
	l = len(m.ValidatorConsPubKey)
	if l > 0 {
		n += 1 + l + sovTypeNodeAccount(uint64(l))
	}
	l = m.Bond.Size()
	n += 1 + l + sovTypeNodeAccount(uint64(l))
	if m.ActiveBlockHeight != 0 {
		n += 1 + sovTypeNodeAccount(uint64(m.ActiveBlockHeight))
	}
	l = len(m.BondAddress)
	if l > 0 {
		n += 1 + l + sovTypeNodeAccount(uint64(l))
	}
	if m.StatusSince != 0 {
		n += 1 + sovTypeNodeAccount(uint64(m.StatusSince))
	}
	if len(m.SignerMembership) > 0 {
		for _, s := range m.SignerMembership {
			l = len(s)
			n += 1 + l + sovTypeNodeAccount(uint64(l))
		}
	}
	if m.RequestedToLeave {
		n += 2
	}
	if m.ForcedToLeave {
		n += 2
	}
	if m.LeaveScore != 0 {
		n += 1 + sovTypeNodeAccount(uint64(m.LeaveScore))
	}
	l = len(m.IPAddress)
	if l > 0 {
		n += 1 + l + sovTypeNodeAccount(uint64(l))
	}
	l = len(m.Version)
	if l > 0 {
		n += 1 + l + sovTypeNodeAccount(uint64(l))
	}
	if m.Type != 0 {
		n += 1 + sovTypeNodeAccount(uint64(m.Type))
	}
	return n
}

func (m *BondProvider) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.BondAddress)
	if l > 0 {
		n += 1 + l + sovTypeNodeAccount(uint64(l))
	}
	l = m.Bond.Size()
	n += 1 + l + sovTypeNodeAccount(uint64(l))
	return n
}

func (m *BondProviders) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.NodeAddress)
	if l > 0 {
		n += 1 + l + sovTypeNodeAccount(uint64(l))
	}
	if len(m.Providers) > 0 {
		for _, e := range m.Providers {
			l = e.Size()
			n += 1 + l + sovTypeNodeAccount(uint64(l))
		}
	}
	return n
}

func sovTypeNodeAccount(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozTypeNodeAccount(x uint64) (n int) {
	return sovTypeNodeAccount(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *BondProvider) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&BondProvider{`,
		`BondAddress:` + fmt.Sprintf("%v", this.BondAddress) + `,`,
		`Bond:` + fmt.Sprintf("%v", this.Bond) + `,`,
		`}`,
	}, "")
	return s
}
func (this *BondProviders) String() string {
	if this == nil {
		return "nil"
	}
	repeatedStringForProviders := "[]BondProvider{"
	for _, f := range this.Providers {
		repeatedStringForProviders += strings.Replace(strings.Replace(f.String(), "BondProvider", "BondProvider", 1), `&`, ``, 1) + ","
	}
	repeatedStringForProviders += "}"
	s := strings.Join([]string{`&BondProviders{`,
		`NodeAddress:` + fmt.Sprintf("%v", this.NodeAddress) + `,`,
		`Providers:` + repeatedStringForProviders + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringTypeNodeAccount(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *NodeAccount) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTypeNodeAccount
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
			return fmt.Errorf("proto: NodeAccount: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: NodeAccount: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field NodeAddress", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
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
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.NodeAddress = append(m.NodeAddress[:0], dAtA[iNdEx:postIndex]...)
			if m.NodeAddress == nil {
				m.NodeAddress = []byte{}
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Status", wireType)
			}
			m.Status = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Status |= NodeStatus(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PubKeySet", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
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
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.PubKeySet.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ValidatorConsPubKey", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ValidatorConsPubKey = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Bond", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Bond.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ActiveBlockHeight", wireType)
			}
			m.ActiveBlockHeight = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ActiveBlockHeight |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field BondAddress", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.BondAddress = gitlab_com_thorchain_thornode_common.Address(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 8:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field StatusSince", wireType)
			}
			m.StatusSince = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.StatusSince |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 9:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SignerMembership", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.SignerMembership = append(m.SignerMembership, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		case 10:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field RequestedToLeave", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.RequestedToLeave = bool(v != 0)
		case 11:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ForcedToLeave", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.ForcedToLeave = bool(v != 0)
		case 12:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field LeaveScore", wireType)
			}
			m.LeaveScore = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.LeaveScore |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 13:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field IPAddress", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.IPAddress = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 14:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Version", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Version = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 15:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Type", wireType)
			}
			m.Type = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Type |= NodeType(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipTypeNodeAccount(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthTypeNodeAccount
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
func (m *BondProvider) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTypeNodeAccount
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
			return fmt.Errorf("proto: BondProvider: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BondProvider: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field BondAddress", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
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
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.BondAddress = append(m.BondAddress[:0], dAtA[iNdEx:postIndex]...)
			if m.BondAddress == nil {
				m.BondAddress = []byte{}
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Bond", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Bond.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTypeNodeAccount(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthTypeNodeAccount
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
func (m *BondProviders) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTypeNodeAccount
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
			return fmt.Errorf("proto: BondProviders: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: BondProviders: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field NodeAddress", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
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
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.NodeAddress = append(m.NodeAddress[:0], dAtA[iNdEx:postIndex]...)
			if m.NodeAddress == nil {
				m.NodeAddress = []byte{}
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Providers", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTypeNodeAccount
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
				return ErrInvalidLengthTypeNodeAccount
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Providers = append(m.Providers, BondProvider{})
			if err := m.Providers[len(m.Providers)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTypeNodeAccount(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthTypeNodeAccount
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthTypeNodeAccount
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
func skipTypeNodeAccount(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowTypeNodeAccount
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
					return 0, ErrIntOverflowTypeNodeAccount
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
					return 0, ErrIntOverflowTypeNodeAccount
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
				return 0, ErrInvalidLengthTypeNodeAccount
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupTypeNodeAccount
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthTypeNodeAccount
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthTypeNodeAccount        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowTypeNodeAccount          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupTypeNodeAccount = fmt.Errorf("proto: unexpected end of group")
)
