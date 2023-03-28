package mdb

const (
	TokenStatusEnable  = 1
	TokenStatusDisable = 2
	TokenTypeTRC20     = 1
	TokenTypeERC20     = 2
)

// WalletAddress  钱包表
type WalletAddress struct {
	Token     string `gorm:"column:token" json:"token"`           //  钱包token
	Status    int64  `gorm:"column:status" json:"status"`         //  1:启用 2:禁用
	TokenType int64  `gorm:"column:token_type" json:"token_type"` //  1:TRC20 2:ERC20
	BaseModel
}

// TableName sets the insert table name for this struct type
func (w *WalletAddress) TableName() string {
	return "usdt_wallet_address"
}
