package domain

// AssetType classifies the settlement layer or protocol of an asset.
type AssetType string

const (
	AssetTypeFiat      AssetType = "FIAT"
	AssetTypeTaproot   AssetType = "TAPROOT"
	AssetTypeLightning AssetType = "LIGHTNING"
)

// Asset represents the identity of a tradeable asset.
// This is strictly identity metadata — it does NOT carry quantity/balance.
type Asset struct {
	ID       string
	Ticker   string
	Name     string
	Decimals int32
	Type     AssetType
}

// AssetBalance pairs an asset identity with a wallet balance.
type AssetBalance struct {
	Asset
	Amount uint64
}
