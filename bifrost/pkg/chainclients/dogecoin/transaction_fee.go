package dogecoin

// TransactionFee on dogecoin
type TransactionFee struct {
	Fee   float64 `json:"fee"`
	VSize int32   `json:"v_size"`
}
