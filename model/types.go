package model

import "database/sql"

// ProductMaster はデータベースから読み込んだ製品マスターのデータを表します。
type ProductMaster struct {
	ProductCode         string  `json:"productCode"`
	YjCode              string  `json:"yjCode"`
	ProductName         string  `json:"productName"`
	Origin              string  `json:"origin"`
	KanaName            string  `json:"kanaName"`
	MakerName           string  `json:"makerName"`
	UsageClassification string  `json:"usageClassification"`
	PackageForm         string  `json:"packageForm"`
	PackageSpec         string  `json:"packageSpec"`
	YjUnitName          string  `json:"yjUnitName"`
	YjPackUnitQty       float64 `json:"yjPackUnitQty"`
	FlagPoison          int     `json:"flagPoison"`
	FlagDeleterious     int     `json:"flagDeleterious"`
	FlagNarcotic        int     `json:"flagNarcotic"`
	FlagPsychotropic    int     `json:"flagPsychotropic"`
	FlagStimulant       int     `json:"flagStimulant"`
	FlagStimulantRaw    int     `json:"flagStimulantRaw"`
	JanPackInnerQty     float64 `json:"janPackInnerQty"`
	JanUnitCode         int     `json:"janUnitCode"`
	JanPackUnitQty      float64 `json:"janPackUnitQty"`
	NhiPrice            float64 `json:"nhiPrice"`
	PurchasePrice       float64 `json:"purchasePrice"`
	SupplierWholesale   string  `json:"supplierWholesale"`
}

// ProductMasterInput はデータベースへの書き込み（作成・更新）に使用する製品マスターのデータを表します。
type ProductMasterInput struct {
	ProductCode         string  `json:"productCode"`
	YjCode              string  `json:"yjCode"`
	ProductName         string  `json:"productName"`
	Origin              string  `json:"origin"`
	KanaName            string  `json:"kanaName"`
	MakerName           string  `json:"makerName"`
	UsageClassification string  `json:"usageClassification"`
	PackageForm         string  `json:"packageForm"`
	PackageSpec         string  `json:"packageSpec"`
	YjUnitName          string  `json:"yjUnitName"`
	YjPackUnitQty       float64 `json:"yjPackUnitQty"`
	FlagPoison          int     `json:"flagPoison"`
	FlagDeleterious     int     `json:"flagDeleterious"`
	FlagNarcotic        int     `json:"flagNarcotic"`
	FlagPsychotropic    int     `json:"flagPsychotropic"`
	FlagStimulant       int     `json:"flagStimulant"`
	FlagStimulantRaw    int     `json:"flagStimulantRaw"`
	JanPackInnerQty     float64 `json:"janPackInnerQty"`
	JanUnitCode         int     `json:"janUnitCode"`
	JanPackUnitQty      float64 `json:"janPackUnitQty"`
	NhiPrice            float64 `json:"nhiPrice"`
	PurchasePrice       float64 `json:"purchasePrice"`
	SupplierWholesale   string  `json:"supplierWholesale"`
}

// JCShms はJCSHMSマスタとJANCODEマスタから必要な情報をまとめた構造体です。
type JCShms struct {
	JC009 string
	JC013 string // 内外区分
	JC018 string
	JC022 string
	JC030 string
	JC037 string
	JC039 string
	JC044 float64
	JC050 float64
	JC061 int
	JC062 int
	JC063 int
	JC064 int
	JC065 int
	JC066 int
	JA006 sql.NullFloat64
	JA007 sql.NullString
	JA008 sql.NullFloat64
}

// TransactionRecord はデータベースに保存される個々の取引の全情報を表します。
type TransactionRecord struct {
	ID                  int            `json:"id"`
	TransactionDate     string         `json:"transactionDate"`
	ClientCode          string         `json:"clientCode"`
	ReceiptNumber       string         `json:"receiptNumber"`
	LineNumber          string         `json:"lineNumber"`
	Flag                int            `json:"flag"`
	JanCode             string         `json:"janCode"`
	YjCode              string         `json:"yjCode"`
	ProductName         string         `json:"productName"`
	KanaName            string         `json:"kanaName"`
	UsageClassification string         `json:"usageClassification"` // <-- New
	PackageForm         string         `json:"packageForm"`
	PackageSpec         string         `json:"packageSpec"`
	MakerName           string         `json:"makerName"`
	DatQuantity         float64        `json:"datQuantity"`
	JanPackInnerQty     float64        `json:"janPackInnerQty"`
	JanQuantity         float64        `json:"janQuantity"`
	JanPackUnitQty      float64        `json:"janPackUnitQty"`
	JanUnitName         string         `json:"janUnitName"`
	JanUnitCode         string         `json:"janUnitCode"`
	YjQuantity          float64        `json:"yjQuantity"`
	YjPackUnitQty       float64        `json:"yjPackUnitQty"`
	YjUnitName          string         `json:"yjUnitName"`
	UnitPrice           float64        `json:"unitPrice"`         // Corresponds to nhi_price
	PurchasePrice       float64        `json:"purchasePrice"`     // <-- New
	SupplierWholesale   string         `json:"supplierWholesale"` // <-- New
	Subtotal            float64        `json:"subtotal"`
	TaxAmount           float64        `json:"taxAmount"`
	TaxRate             float64        `json:"taxRate"`
	ExpiryDate          string         `json:"expiryDate"`
	LotNumber           string         `json:"lotNumber"`
	FlagPoison          int            `json:"flagPoison"`
	FlagDeleterious     int            `json:"flagDeleterious"`
	FlagNarcotic        int            `json:"flagNarcotic"`
	FlagPsychotropic    int            `json:"flagPsychotropic"`
	FlagStimulant       int            `json:"flagStimulant"`
	FlagStimulantRaw    int            `json:"flagStimulantRaw"`
	ProcessFlagMA       string         `json:"processFlagMA"`
	ProcessingStatus    sql.NullString `json:"processingStatus"`
}

// ProductMasterView is a data structure for the master edit screen, including formatted fields.
type ProductMasterView struct {
	ProductMaster
	FormattedPackageSpec string `json:"formattedPackageSpec"`
}

// Client is a data structure for a client record.
type Client struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// AggregationFilters holds the filter criteria for the stock ledger report.
type AggregationFilters struct {
	StartDate   string
	EndDate     string
	KanaName    string
	DrugTypes   []string
	NoMovement  bool
	Coefficient float64
}

// StockLedgerYJGroup represents a top-level grouping by YJ code in the stock ledger.
type StockLedgerYJGroup struct {
	YjCode            string                    `json:"yjCode"`
	ProductName       string                    `json:"productName"`
	YjUnitName        string                    `json:"yjUnitName"`
	PackageLedgers    []StockLedgerPackageGroup `json:"packageLedgers"`
	StartingBalance   float64                   `json:"startingBalance"`
	NetChange         float64                   `json:"netChange"`
	EndingBalance     float64                   `json:"endingBalance"`
	TotalReorderPoint float64                   `json:"totalReorderPoint"`
	IsReorderNeeded   bool                      `json:"isReorderNeeded"`
}

// StockLedgerPackageGroup represents a sub-grouping by package specification.
type StockLedgerPackageGroup struct {
	PackageKey      string              `json:"packageKey"`
	JanUnitName     string              `json:"janUnitName"`
	StartingBalance float64             `json:"startingBalance"`
	Transactions    []LedgerTransaction `json:"transactions"`
	NetChange       float64             `json:"netChange"`
	EndingBalance   float64             `json:"endingBalance"`
	MaxUsage        float64             `json:"maxUsage"`
	ReorderPoint    float64             `json:"reorderPoint"`
	IsReorderNeeded bool                `json:"isReorderNeeded"`
}

// LedgerTransaction is a transaction record that includes a running balance.
type LedgerTransaction struct {
	TransactionRecord
	RunningBalance float64 `json:"runningBalance"`
}

// UnifiedInputRecord is a superset structure that can hold data from any input source (DAT, USAGE, INV).
// The parsers' job is to create slices of this struct.
type UnifiedInputRecord struct {
	// Common Fields
	Date        string `json:"date"`
	JanCode     string `json:"janCode"`
	YjCode      string `json:"yjCode"`
	ProductName string `json:"productName"`

	// Quantity Fields
	DatQuantity     float64 `json:"datQuantity"`
	JanPackInnerQty float64 `json:"janPackInnerQty"`
	JanQuantity     float64 `json:"janQuantity"`
	YjQuantity      float64 `json:"yjQuantity"`

	// Unit/Spec Fields
	YjUnitName string `json:"yjUnitName"`

	// DAT-specific Fields
	ClientCode    string  `json:"clientCode"`
	ReceiptNumber string  `json:"receiptNumber"`
	LineNumber    string  `json:"lineNumber"`
	Flag          int     `json:"flag"`
	UnitPrice     float64 `json:"unitPrice"`
	Subtotal      float64 `json:"subtotal"`
	ExpiryDate    string  `json:"expiryDate"`
	LotNumber     string  `json:"lotNumber"`
}
