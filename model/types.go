// C:\Dev\WASABI\model\types.go

package model

import "database/sql"

// (Struct definitions up to DeadStockFilters are unchanged)
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
type JCShms struct {
	JC009 string
	JC013 string
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
	UsageClassification string         `json:"usageClassification"`
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
	UnitPrice           float64        `json:"unitPrice"`
	PurchasePrice       float64        `json:"purchasePrice"`
	SupplierWholesale   string         `json:"supplierWholesale"`
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

func (t *TransactionRecord) SignedYjQty() float64 {
	switch t.Flag {
	case 1, 4, 11:
		return t.YjQuantity
	case 2, 3, 5, 12:
		return -t.YjQuantity
	default:
		return 0
	}
}
func (t *TransactionRecord) ToProductMaster() *ProductMaster {
	return &ProductMaster{
		ProductCode:         t.JanCode,
		YjCode:              t.YjCode,
		ProductName:         t.ProductName,
		KanaName:            t.KanaName,
		UsageClassification: t.UsageClassification,
		PackageForm:         t.PackageForm,
		JanPackInnerQty:     t.JanPackInnerQty,
		YjUnitName:          t.YjUnitName,
	}
}

type ProductMasterView struct {
	ProductMaster
	FormattedPackageSpec string `json:"formattedPackageSpec"`
}
type Client struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type AggregationFilters struct {
	StartDate   string
	EndDate     string
	KanaName    string
	DrugTypes   []string
	DosageForm  string
	Coefficient float64
}
type StockLedgerYJGroup struct {
	YjCode                string                    `json:"yjCode"`
	ProductName           string                    `json:"productName"`
	YjUnitName            string                    `json:"yjUnitName"`
	PackageLedgers        []StockLedgerPackageGroup `json:"packageLedgers"`
	StartingBalance       interface{}               `json:"startingBalance"`
	NetChange             float64                   `json:"netChange"`
	EndingBalance         interface{}               `json:"endingBalance"`
	TotalReorderPoint     float64                   `json:"totalReorderPoint"`
	IsReorderNeeded       bool                      `json:"isReorderNeeded"`
	TotalBaseReorderPoint float64                   `json:"totalBaseReorderPoint"`
	TotalPrecompounded    float64                   `json:"totalPrecompounded"`
}
type StockLedgerPackageGroup struct {
	PackageKey         string              `json:"packageKey"`
	JanUnitName        string              `json:"janUnitName"`
	StartingBalance    interface{}         `json:"startingBalance"`
	Transactions       []LedgerTransaction `json:"transactions"`
	NetChange          float64             `json:"netChange"`
	EndingBalance      interface{}         `json:"endingBalance"`
	MaxUsage           float64             `json:"maxUsage"`
	ReorderPoint       float64             `json:"reorderPoint"`
	IsReorderNeeded    bool                `json:"isReorderNeeded"`
	Master             *ProductMaster      `json:"-"`
	BaseReorderPoint   float64             `json:"baseReorderPoint"`
	PrecompoundedTotal float64             `json:"precompoundedTotal"`
}
type LedgerTransaction struct {
	TransactionRecord
	RunningBalance float64 `json:"runningBalance"`
}
type UnifiedInputRecord struct {
	Date            string  `json:"date"`
	JanCode         string  `json:"janCode"`
	YjCode          string  `json:"yjCode"`
	ProductName     string  `json:"productName"`
	DatQuantity     float64 `json:"datQuantity"`
	JanPackInnerQty float64 `json:"janPackInnerQty"`
	JanQuantity     float64 `json:"janQuantity"`
	YjQuantity      float64 `json:"yjQuantity"`
	YjUnitName      string  `json:"yjUnitName"`
	ClientCode      string  `json:"clientCode"`
	ReceiptNumber   string  `json:"receiptNumber"`
	LineNumber      string  `json:"lineNumber"`
	Flag            int     `json:"flag"`
	UnitPrice       float64 `json:"unitPrice"`
	Subtotal        float64 `json:"subtotal"`
	ExpiryDate      string  `json:"expiryDate"`
	LotNumber       string  `json:"lotNumber"`
}
type DeadStockGroup struct {
	YjCode      string             `json:"yjCode"`
	ProductName string             `json:"productName"`
	TotalStock  float64            `json:"totalStock"`
	Packages    []DeadStockPackage `json:"packages"`
}
type DeadStockPackage struct {
	ProductMaster
	CurrentStock float64           `json:"currentStock"`
	SavedRecords []DeadStockRecord `json:"savedRecords"`
}
type DeadStockRecord struct {
	ID               int     `json:"id"`
	ProductCode      string  `json:"productCode"`
	YjCode           string  `json:"yjCode"`
	PackageForm      string  `json:"packageForm"`
	JanPackInnerQty  float64 `json:"janPackInnerQty"`
	YjUnitName       string  `json:"yjUnitName"`
	StockQuantityJan float64 `json:"stockQuantityJan"`
	ExpiryDate       string  `json:"expiryDate"`
	LotNumber        string  `json:"lotNumber"`
}

// ▼▼▼ [修正点] DeadStockFiltersにCoefficientを追加 ▼▼▼
type DeadStockFilters struct {
	StartDate        string
	EndDate          string
	ExcludeZeroStock bool
	Coefficient      float64
}

// ▲▲▲ 修正ここまで ▲▲▲
type PreCompoundingRecord struct {
	ID            int     `json:"id"`
	PatientNumber string  `json:"patientNumber"`
	ProductCode   string  `json:"productCode"`
	Quantity      float64 `json:"quantity"` // YJ Quantity
	CreatedAt     string  `json:"createdAt"`
}
