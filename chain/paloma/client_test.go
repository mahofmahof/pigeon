package paloma

import (
	"context"
	"errors"
	"net"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/gogoproto/proto"
	consensus "github.com/palomachain/paloma/x/consensus/types"
	consensusmocks "github.com/palomachain/paloma/x/consensus/types/mocks"
	valset "github.com/palomachain/paloma/x/valset/types"
	valsetmocks "github.com/palomachain/paloma/x/valset/types/mocks"
	"github.com/palomachain/pigeon/chain"
	clientmocks "github.com/palomachain/pigeon/chain/paloma/mocks"
	"github.com/palomachain/pigeon/types/testdata"
	"github.com/strangelove-ventures/lens/byop"
	lens "github.com/strangelove-ventures/lens/client"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

var errTestErr = errors.New("sample error")

func consensusQueryServerDialer(t *testing.T, msgsrv *consensusmocks.QueryServer) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)

	server := grpc.NewServer()

	consensus.RegisterQueryServer(server, msgsrv)

	go func() {
		err := server.Serve(listener)
		require.NoError(t, err)
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

func valsetQueryServerDailer(t *testing.T, msgsrv *valsetmocks.QueryServer) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)

	server := grpc.NewServer()

	valset.RegisterQueryServer(server, msgsrv)

	go func() {
		err := server.Serve(listener)
		require.NoError(t, err)
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

func makeCodec() lens.Codec {
	return lens.MakeCodec([]module.AppModuleBasic{
		byop.Module{
			ModuleName: "testing",
			MsgsImplementations: []byop.RegisterImplementation{
				{
					Iface: (*consensus.ConsensusMsg)(nil),
					Msgs: []proto.Message{
						&testdata.SimpleMessage{},
						&testdata.SimpleMessage2{},
					},
				},
			},
		},
	}, []string{})
}

func TestQueryingMessagesForSigning(t *testing.T) {
	codec := makeCodec()
	for _, tt := range []struct {
		name   string
		mcksrv func(*testing.T) *consensusmocks.QueryServer
		expRes []chain.QueuedMessage
		expErr error

		// used only for testing the GRPC responses because GRPC is doing a
		// string concatenation on errors, thus we can't do proper error
		// inspection
		expectsAnyError bool
	}{
		{
			name:   "called with correct arguments",
			expRes: []chain.QueuedMessage{},
			mcksrv: func(t *testing.T) *consensusmocks.QueryServer {
				srv := consensusmocks.NewQueryServer(t)
				srv.On("QueuedMessagesForSigning", mock.Anything, &consensus.QueryQueuedMessagesForSigningRequest{
					ValAddress:    sdk.ValAddress("validator"),
					QueueTypeName: "queueName",
				}).Return(
					&consensus.QueryQueuedMessagesForSigningResponse{
						MessageToSign: []*consensus.MessageToSign{},
					},
					nil,
				).Once()
				return srv
			},
		},
		{
			name: "messages are happily returned",
			mcksrv: func(t *testing.T) *consensusmocks.QueryServer {
				srv := consensusmocks.NewQueryServer(t)
				srv.On("QueuedMessagesForSigning", mock.Anything, mock.Anything).Return(
					&consensus.QueryQueuedMessagesForSigningResponse{
						MessageToSign: []*consensus.MessageToSign{
							{
								Nonce:       []byte("nonce-123"),
								Id:          456,
								BytesToSign: []byte("bla"),
							},
							{
								Nonce:       []byte("nonce-321"),
								Id:          654,
								BytesToSign: []byte("bla2"),
							},
						},
					},
					nil,
				).Once()
				return srv
			},
			expRes: []chain.QueuedMessage{
				{
					Nonce:       []byte("nonce-123"),
					ID:          456,
					BytesToSign: []byte("bla"),
				},
				{
					Nonce:       []byte("nonce-321"),
					ID:          654,
					BytesToSign: []byte("bla2"),
				},
			},
		},
		{
			name: "client returns an error",
			mcksrv: func(t *testing.T) *consensusmocks.QueryServer {
				srv := consensusmocks.NewQueryServer(t)
				srv.On("QueuedMessagesForSigning", mock.Anything, mock.Anything).Return(
					nil,
					errTestErr,
				).Once()
				return srv
			},
			expectsAnyError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// setting everything up
			ctx := context.Background()
			mocksrv := tt.mcksrv(t)
			conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(consensusQueryServerDialer(t, mocksrv)))
			require.NoError(t, err)

			// setup complete
			// calling the function that we are testing
			msgs, err := queryMessagesForSigning(
				ctx,
				conn,
				codec.Marshaler,
				sdk.ValAddress("validator"),
				"queueName",
			)
			if tt.expectsAnyError {
				require.Error(t, err)
			} else {
				require.ErrorIs(t, err, tt.expErr)
			}
			require.Equal(t, tt.expRes, msgs)
		})
	}
}

// TODO : Break into the different queues
//func TestGetMessagesInQueue(t *testing.T) {
//	codec := makeCodec()
//	for _, tt := range []struct {
//		name   string
//		mcksrv func(*testing.T) *consensusmocks.QueryServer
//		expRes []chain.QueuedMessage
//
//		expMsgs         []chain.MessageWithSignatures
//		expectsAnyError bool
//	}{
//		{
//			name: "happy path",
//			mcksrv: func(t *testing.T) *consensusmocks.QueryServer {
//				srv := consensusmocks.NewQueryServer(t)
//				srv.On("MessagesInQueue", mock.Anything, mock.Anything).Return(&consensus.QueryMessagesInQueueResponse{
//					Messages: []*consensus.MessageWithSignatures{
//						{
//							Nonce: []byte("hello"),
//							Id:    123,
//							Msg: whoops.Must(codectypes.NewAnyWithValue(&testdata.SimpleMessage{
//								Hello: "bob",
//							})),
//							SignData: []*consensus.ValidatorSignature{
//								{
//									ValAddress: sdk.ValAddress("abc"),
//									Signature:  []byte("sig-123"),
//								},
//								{
//									ValAddress: sdk.ValAddress("def"),
//									Signature:  []byte("sig-456"),
//								},
//							},
//						},
//						{
//							Nonce: []byte("hello2"),
//							Id:    456,
//							Msg: whoops.Must(codectypes.NewAnyWithValue(&testdata.SimpleMessage{
//								Hello: "alice",
//							})),
//							SignData: []*consensus.ValidatorSignature{
//								{
//									ValAddress: sdk.ValAddress("abc"),
//									Signature:  []byte("sig-123"),
//								},
//							},
//						},
//					},
//				}, nil).Once()
//				return srv
//			},
//			expMsgs: []chain.MessageWithSignatures{
//				{
//					QueuedMessage: chain.QueuedMessage{
//						Nonce: []byte("hello"),
//						EventNonce:    123,
//						Msg: &testdata.SimpleMessage{
//							Hello: "bob",
//						},
//					},
//					Signatures: []chain.ValidatorSignature{
//						{
//							ValAddress: sdk.ValAddress("abc"),
//							Signature:  []byte("sig-123"),
//						},
//						{
//							ValAddress: sdk.ValAddress("def"),
//							Signature:  []byte("sig-456"),
//						},
//					},
//				},
//				{
//					QueuedMessage: chain.QueuedMessage{
//						Nonce: []byte("hello2"),
//						EventNonce:    456,
//						Msg: &testdata.SimpleMessage{
//							Hello: "alice",
//						},
//					},
//					Signatures: []chain.ValidatorSignature{
//						{
//							ValAddress: sdk.ValAddress("abc"),
//							Signature:  []byte("sig-123"),
//						},
//					},
//				},
//			},
//		},
//	} {
//		t.Run(tt.name, func(t *testing.T) {
//			// setting everything up
//			ctx := context.Background()
//			mocksrv := tt.mcksrv(t)
//			conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(consensusQueryServerDialer(t, mocksrv)))
//			require.NoError(t, err)
//
//			msgs, err := queryMessagesInQueue(ctx, "bob", nil, conn, codec.Marshaler)
//
//			require.Equal(t, tt.expMsgs, msgs)
//
//			if tt.expectsAnyError {
//				require.Error(t, err)
//			} else {
//				require.NoError(t, err)
//			}
//		})
//	}
//}

func TestQueryValidatorInfo(t *testing.T) {
	fakeErr := errors.New("something")
	fakeExternalInfo := []*valset.ExternalChainInfo{
		{
			ChainType:        "abc",
			ChainReferenceID: "123",
			Address:          "123",
			Pubkey:           []byte("abc"),
		},
	}
	for _, tt := range []struct {
		name   string
		mcksrv func(*testing.T) *valsetmocks.QueryServer
		expRes []chain.QueuedMessage

		expectedChainInfo []*valset.ExternalChainInfo
		expectsAnyError   bool
	}{
		{
			name: "happy path",
			mcksrv: func(t *testing.T) *valsetmocks.QueryServer {
				srv := valsetmocks.NewQueryServer(t)
				srv.On("ValidatorInfo", mock.Anything, mock.Anything).Return(&valset.QueryValidatorInfoResponse{
					ChainInfos: fakeExternalInfo,
				}, nil).Once()
				return srv
			},
			expectedChainInfo: fakeExternalInfo,
		},
		{
			name: "grpc returns error",
			mcksrv: func(t *testing.T) *valsetmocks.QueryServer {
				srv := valsetmocks.NewQueryServer(t)
				srv.On("ValidatorInfo", mock.Anything, mock.Anything).Return(nil, fakeErr).Once()
				return srv
			},
			expectsAnyError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// setting everything up
			ctx := context.Background()
			mocksrv := tt.mcksrv(t)
			conn, err := grpc.DialContext(ctx, "", grpc.WithInsecure(), grpc.WithContextDialer(valsetQueryServerDailer(t, mocksrv)))
			require.NoError(t, err)

			client := Client{
				GRPCClient:     conn,
				creatorValoper: "bla",
			}
			externalChainInfos, err := client.QueryValidatorInfo(ctx)

			require.Equal(t, tt.expectedChainInfo, externalChainInfos)

			if tt.expectsAnyError {
				require.Error(t, err)
			}
		})
	}
}

func TestAddingExternalChainInfo(t *testing.T) {
	fakeErr := errors.New("something")
	for _, tt := range []struct {
		name      string
		chainInfo []ChainInfoIn
		mcksrv    func(*testing.T) *clientmocks.MessageSender
		expRes    []chain.QueuedMessage

		expectsAnyError bool
	}{
		{
			name:      "without chain infos provided does nothing",
			chainInfo: []ChainInfoIn{},
			mcksrv: func(t *testing.T) *clientmocks.MessageSender {
				srv := clientmocks.NewMessageSender(t)
				t.Cleanup(func() {
					srv.AssertNotCalled(t, "SendMsg", mock.Anything, mock.Anything)
				})
				return srv
			},
		},
		{
			name: "happy path",
			chainInfo: []ChainInfoIn{
				{ChainReferenceID: "chain1", AccAddress: "addr1", ChainType: "chain-type-1", PubKey: []byte("pk1")},
				{ChainReferenceID: "chain2", AccAddress: "addr2", ChainType: "chain-type-2", PubKey: []byte("pk2")},
			},
			mcksrv: func(t *testing.T) *clientmocks.MessageSender {
				srv := clientmocks.NewMessageSender(t)
				srv.On(
					"SendMsg",
					mock.Anything,
					&valset.MsgAddExternalChainInfoForValidator{
						ChainInfos: []*valset.ExternalChainInfo{
							{ChainReferenceID: "chain1", Address: "addr1", ChainType: "chain-type-1", Pubkey: []byte("pk1")},
							{ChainReferenceID: "chain2", Address: "addr2", ChainType: "chain-type-2", Pubkey: []byte("pk2")},
						},
					},
					"",
				).Return(nil, nil).Once()
				return srv
			},
		},
		{
			name: "with SendMsg returning errors",
			chainInfo: []ChainInfoIn{
				{ChainReferenceID: "chain1", AccAddress: "addr1", ChainType: "chain-type-1", PubKey: []byte("pk1")},
				{ChainReferenceID: "chain2", AccAddress: "addr2", ChainType: "chain-type-2", PubKey: []byte("pk2")},
			},
			mcksrv: func(t *testing.T) *clientmocks.MessageSender {
				srv := clientmocks.NewMessageSender(t)
				srv.On("SendMsg", mock.Anything, mock.Anything, "").Return(nil, fakeErr).Once()
				return srv
			},
			expectsAnyError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// setting everything up
			ctx := context.Background()
			mocksrv := tt.mcksrv(t)

			client := Client{
				MessageSender: mocksrv,
			}
			err := client.AddExternalChainInfo(
				ctx,
				tt.chainInfo...,
			)

			if tt.expectsAnyError {
				require.Error(t, err)
			}
		})
	}
}

func TestKeepValidatorAlive(t *testing.T) {
	creator := "bob"

	testcases := []struct {
		name        string
		appVersion  string
		setup       func() MessageSender
		expectedErr error
	}{
		{
			name:       "sends keepalive message formatted as expected",
			appVersion: "v1.3.0",
			setup: func() MessageSender {
				msgSender := clientmocks.NewMessageSender(t)
				msgSender.On("SendMsg",
					mock.Anything, &valset.MsgKeepAlive{
						Creator:       creator,
						PigeonVersion: "v1.3.0",
					},
					"",
				).Return(nil, nil)
				return msgSender
			},
		},
		{
			name: "returns error when message sender has an error",
			setup: func() MessageSender {
				msgSender := clientmocks.NewMessageSender(t)
				msgSender.On("SendMsg", mock.Anything, mock.Anything, "").Return(nil, errTestErr)
				return msgSender
			},
			expectedErr: errTestErr,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			msgSender := tt.setup()
			ctx := context.Background()

			client := Client{
				creator:       creator,
				MessageSender: msgSender,
			}

			err := client.KeepValidatorAlive(ctx, tt.appVersion)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestBroadcastingMessageSignatures(t *testing.T) {
	ctx := context.Background()
	creator := "bob"
	for _, tt := range []struct {
		name       string
		setup      func() MessageSender
		signatures []BroadcastMessageSignatureIn

		expErr error
	}{
		{
			name: "nothing happens when there are no signatures being sent",
			setup: func() MessageSender {
				return clientmocks.NewMessageSender(t)
			},
		},
		{
			name: "signatures are sent over",
			signatures: []BroadcastMessageSignatureIn{
				{
					ID:            123,
					QueueTypeName: "abc",
					Signature:     []byte(`sig-123`),
				},
				{
					ID:            456,
					QueueTypeName: "def",
					Signature:     []byte(`sig-789`),
				},
			},
			setup: func() MessageSender {
				msgSender := clientmocks.NewMessageSender(t)
				expectedSignaturesMsg := &consensus.MsgAddMessagesSignatures{
					Creator: creator,
					SignedMessages: []*consensus.ConsensusMessageSignature{
						{
							Id:            123,
							QueueTypeName: "abc",
							Signature:     []byte(`sig-123`),
						},
						{
							Id:            456,
							QueueTypeName: "def",
							Signature:     []byte(`sig-789`),
						},
					},
				}
				msgSender.On("SendMsg", mock.Anything, expectedSignaturesMsg, "").Return(nil, nil)
				return msgSender
			},
		},
		{
			name: "msg sender returns an error",
			setup: func() MessageSender {
				msgSender := clientmocks.NewMessageSender(t)
				msgSender.On("SendMsg", mock.Anything, mock.Anything, "").Return(nil, errTestErr)
				return msgSender
			},
			signatures: []BroadcastMessageSignatureIn{
				{},
			},
			expErr: errTestErr,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			msgSender := tt.setup()
			err := broadcastMessageSignatures(
				ctx,
				msgSender,
				creator,
				tt.signatures...,
			)
			require.ErrorIs(t, err, tt.expErr)
		})
	}
}
