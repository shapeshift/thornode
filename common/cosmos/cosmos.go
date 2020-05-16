package common

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	NewUint               = sdk.NewUint
	ParseUint             = sdk.ParseUint
	NewInt                = sdk.NewInt
	NewDec                = sdk.NewDec
	ZeroUint              = sdk.ZeroUint
	ZeroDec               = sdk.ZeroDec
	OneUint               = sdk.OneUint
	NewCoin               = sdk.NewCoin
	NewCoins              = sdk.NewCoins
	ParseCoins            = sdk.ParseCoins
	NewUintFromString     = sdk.NewUintFromString
	NewDecWithPrec        = sdk.NewDecWithPrec
	NewDecFromBigInt      = sdk.NewDecFromBigInt
	NewIntFromBigInt      = sdk.NewIntFromBigInt
	NewUintFromBigInt     = sdk.NewUintFromBigInt
	GetAccPubKeyBech32    = sdk.GetAccPubKeyBech32
	Bech32ifyAccPub       = sdk.Bech32ifyAccPub
	AccAddressFromBech32  = sdk.AccAddressFromBech32
	GetFromBech32         = sdk.GetFromBech32
	NewAttribute          = sdk.NewAttribute
	NewDecFromStr         = sdk.NewDecFromStr
	Bech32ifyConsPub      = sdk.Bech32ifyConsPub
	GetConfig             = sdk.GetConfig
	NewEvent              = sdk.NewEvent
	RegisterCodec         = sdk.RegisterCodec
	GetConsPubKeyBech32   = sdk.GetConsPubKeyBech32
	NewError              = sdk.NewError
	NewEventManager       = sdk.NewEventManager
	EventTypeMessage      = sdk.EventTypeMessage
	AttributeKeyModule    = sdk.AttributeKeyModule
	KVStorePrefixIterator = sdk.KVStorePrefixIterator
	NewKVStoreKey         = sdk.NewKVStoreKey
	NewTransientStoreKey  = sdk.NewTransientStoreKey
	StoreTypeTransient    = sdk.StoreTypeTransient
	StoreTypeIAVL         = sdk.StoreTypeIAVL
	NewContext            = sdk.NewContext

	ErrInternal          = sdk.ErrInternal
	ErrUnauthorized      = sdk.ErrUnauthorized
	ErrUnknownRequest    = sdk.ErrUnknownRequest
	ErrInvalidCoins      = sdk.ErrInvalidCoins
	ErrInvalidAddress    = sdk.ErrInvalidAddress
	ErrInsufficientCoins = sdk.ErrInsufficientCoins
	MustSortJSON         = sdk.MustSortJSON

	CodeOK                = sdk.CodeOK
	CodeUnauthorized      = sdk.CodeUnauthorized
	CodeInsufficientFunds = sdk.CodeInsufficientFunds
	CodeInvalidAddress    = sdk.CodeInvalidAddress
	CodeUnknownRequest    = sdk.CodeUnknownRequest
	CodeInternal          = sdk.CodeInternal
	CodeInvalidCoins      = sdk.CodeInvalidCoins
)

type (
	Context       = sdk.Context
	Uint          = sdk.Uint
	Coin          = sdk.Coin
	Coins         = sdk.Coins
	AccAddress    = sdk.AccAddress
	Attribute     = sdk.Attribute
	Error         = sdk.Error
	Result        = sdk.Result
	Event         = sdk.Event
	Events        = sdk.Events
	Dec           = sdk.Dec
	CodeType      = sdk.CodeType
	CodespaceType = sdk.CodespaceType
	Msg           = sdk.Msg
	Iterator      = sdk.Iterator
	Handler       = sdk.Handler
	StoreKey      = sdk.StoreKey
	Querier       = sdk.Querier
)
