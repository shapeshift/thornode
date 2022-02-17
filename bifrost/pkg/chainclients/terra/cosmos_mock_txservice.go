package terra

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"

	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	grpc "google.golang.org/grpc"
)

// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type MockTxServiceClient interface {
	// Simulate simulates executing a transaction for estimating gas usage.
	Simulate(ctx context.Context, in *txtypes.SimulateRequest, opts ...grpc.CallOption) (*txtypes.SimulateResponse, error)
	// GetTx fetches a tx by hash.
	GetTx(ctx context.Context, in *txtypes.GetTxRequest, opts ...grpc.CallOption) (*txtypes.GetTxResponse, error)
	// BroadcastTx broadcast transaction.
	BroadcastTx(ctx context.Context, in *txtypes.BroadcastTxRequest, opts ...grpc.CallOption) (*txtypes.BroadcastTxResponse, error)
	// GetTxsEvent fetches txs by event.
	GetTxsEvent(ctx context.Context, in *txtypes.GetTxsEventRequest, opts ...grpc.CallOption) (*txtypes.GetTxsEventResponse, error)
}

type mockTxServiceClient struct{}

func NewMockTxServiceClient() MockTxServiceClient {
	return &mockTxServiceClient{}
}

func (m *mockTxServiceClient) Simulate(ctx context.Context, in *txtypes.SimulateRequest, opts ...grpc.CallOption) (*txtypes.SimulateResponse, error) {
	return nil, nil
}

func (m *mockTxServiceClient) BroadcastTx(ctx context.Context, in *txtypes.BroadcastTxRequest, opts ...grpc.CallOption) (*txtypes.BroadcastTxResponse, error) {
	return nil, nil
}

func (m *mockTxServiceClient) GetTxsEvent(ctx context.Context, in *txtypes.GetTxsEventRequest, opts ...grpc.CallOption) (*txtypes.GetTxsEventResponse, error) {
	return nil, nil
}

func (m *mockTxServiceClient) GetTx(ctx context.Context, in *txtypes.GetTxRequest, opts ...grpc.CallOption) (*txtypes.GetTxResponse, error) {
	out := new(txtypes.GetTxResponse)

	var err error
	switch strings.ToUpper(in.Hash) {
	case "448DE9B1DEB0B6A8A5B66F760D4EC54A9FE7F4DE2A1422F373327AB6A58EB25B":
		err = unmarshalTxJSONToPb("./test-data/tx_448DE9B1DEB0B6A8A5B66F760D4EC54A9FE7F4DE2A1422F373327AB6A58EB25B.json", out)
	case "FD95AFE5D3C53479E0D12E74CBD1C4EE832AF69FDC6C45A4CA72815D5FBA7B1B":
		err = unmarshalTxJSONToPb("./test-data/tx_FD95AFE5D3C53479E0D12E74CBD1C4EE832AF69FDC6C45A4CA72815D5FBA7B1B.json", out)
	default:
		return nil, fmt.Errorf("unable to find txhash %s", in.Hash)
	}

	if err != nil {
		return nil, err
	}

	return out, nil
}

func unmarshalTxJSONToPb(filePath string, msg proto.Message) error {
	jsonFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	err = jsonpb.Unmarshal(jsonFile, msg)

	if err != nil {
		return err
	}

	return nil
}
