// C:\Users\wasab\OneDrive\デスクトップ\WASABI\model\types.go
package model

import "database/sql"

// ProductMaster は製品マスターの完全なデータ構造です。(tkrから移植)
type ProductMaster struct {
	ProductCode         string  `json:"productCode"`
	YjCode              string  `json:"yjCode"`
	ProductName         string  `json:"productName"`
	KanaName            string  `json:"kanaName"`
	MakerName           string  `json:"makerName"`
	Gs1Code             string  `json:"gs1Code"`
	PackageForm         string  `json:"packageForm"`
	Specification       string  `json:"specification"`
	UsageClassification string  `json:"usageClassification"`
	YjUnitName          string  `json:"yjUnitName"`
	YjPackUnitQty       float64 `json:"yjPackUnitQty"`
	JanPackInnerQty     float64 `json:"janPackInnerQty"`
	JanUnitCode         int     `json:"janUnitCode"`
	JanPackUnitQty      float64 `json:"janPackUnitQty"`
	Origin              string  `json:"origin"`
	NhiPrice            float64 `json:"nhiPrice"`
	PurchasePrice       float64 `json:"purchasePrice"`
	FlagPoison          int     `json:"flagPoison"`
	FlagDeleterious     int     `json:"flagDeleterious"`
	FlagNarcotic        int     `json:"flagNarcotic"`
	FlagPsychotropic    int     `json:"flagPsychotropic"`
	FlagStimulant       int     `json:"flagStimulant"`
	FlagStimulantRaw    int     `json:"flagStimulantRaw"`
	IsOrderStopped      int     `json:"isOrderStopped"`
	SupplierWholesale   string  `json:"supplierWholesale"`
	GroupCode           string  `json:"groupCode"`
	ShelfNumber         string  `json:"shelfNumber"`
	Category            string  `json:"category"`
	UserNotes           string  `json:"userNotes"`
}

// ProductMasterInput は製品マスターを登録・更新する際の入力データ構造です。(tkrから移植)
type ProductMasterInput struct {
	ProductCode         string  `json:"productCode"`
	YjCode              string  `json:"yjCode"`
	ProductName         string  `json:"productName"`
	KanaName            string  `json:"kanaName"`
	MakerName           string  `json:"makerName"`
	Gs1Code             string  `json:"gs1Code"`
	PackageForm         string  `json:"packageForm"`
	Specification       string  `json:"specification"`
	UsageClassification string  `json:"usageClassification"`
	YjUnitName          string  `json:"yjUnitName"`
	YjPackUnitQty       float64 `json:"yjPackUnitQty"`
	JanPackInnerQty     float64 `json:"janPackInnerQty"`
	JanUnitCode         int     `json:"janUnitCode"`
	JanPackUnitQty      float64 `json:"janPackUnitQty"`
	Origin              string  `json:"origin"`
	NhiPrice            float64 `json:"nhiPrice"`
	PurchasePrice       float64 `json:"purchasePrice"`
	FlagPoison          int     `json:"flagPoison"`
	FlagDeleterious     int     `json:"flagDeleterious"`
	FlagNarcotic        int     `json:"flagNarcotic"`
	FlagPsychotropic    int     `json:"flagPsychotropic"`
	FlagStimulant       int     `json:"flagStimulant"`
	FlagStimulantRaw    int     `json:"flagStimulantRaw"`
	IsOrderStopped      int     `json:"isOrderStopped"`
	SupplierWholesale   string  `json:"supplierWholesale"`
	GroupCode           string  `json:"groupCode"`
	ShelfNumber         string  `json:"shelfNumber"`
	Category            string  `json:"category"`
	UserNotes           string  `json:"userNotes"`
}

// (以下はWASABIに元々あった型定義)
type JCShms struct {
	JC009 string
	JC013 string
	JC018 string
	JC020 string
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
	JC122 string
	JA006 sql.NullFloat64
	JA007 sql.NullString
	JA008 sql.NullFloat64
}

type ValuationPackageDetail struct {
	ProductCode   string  `json:"productCode"`
	PackageSpec   string  `json:"packageSpec"`
	Stock         float64 `json:"stock"`
	NhiPrice      float64 `json:"nhiPrice"`
	PurchasePrice float64 `json:"purchasePrice"`
}

type TransactionRecord struct {
	ID                  int     `json:"id"`
	TransactionDate     string  `json:"transactionDate"`
	ClientCode          string  `json:"clientCode"`
	ReceiptNumber       string  `json:"receiptNumber"`
	LineNumber          string  `json:"lineNumber"`
	Flag                int     `json:"flag"`
	JanCode             string  `json:"janCode"`
	YjCode              string  `json:"yjCode"`
	ProductName         string  `json:"productName"`
	KanaName            string  `json:"kanaName"`
	UsageClassification string  `json:"usageClassification"`
	PackageForm         string  `json:"packageForm"`
	PackageSpec         string  `json:"packageSpec"`
	MakerName           string  `json:"makerName"`
	DatQuantity         float64 `json:"datQuantity"`
	JanPackInnerQty     float64 `json:"janPackInnerQty"`
	JanQuantity         float64 `json:"janQuantity"`
	JanPackUnitQty      float64 `json:"janPackUnitQty"`
	JanUnitName         string  `json:"janUnitName"`
	JanUnitCode         string  `json:"janUnitCode"`
	YjQuantity          float64 `json:"yjQuantity"`
	YjPackUnitQty       float64 `json:"yjPackUnitQty"`
	YjUnitName          string  `json:"yjUnitName"`
	UnitPrice           float64 `json:"unitPrice"`
	PurchasePrice       float64 `json:"purchasePrice"`
	SupplierWholesale   string  `json:"supplierWholesale"`
	Subtotal            float64 `json:"subtotal"`
	TaxAmount           float64 `json:"taxAmount"`
	TaxRate             float64 `json:"taxRate"`
	ExpiryDate          string  `json:"expiryDate"`
	LotNumber           string  `json:"lotNumber"`
	FlagPoison          int     `json:"flagPoison"`
	FlagDeleterious     int     `json:"flagDeleterious"`
	FlagNarcotic        int     `json:"flagNarcotic"`
	FlagPsychotropic    int     `json:"flagPsychotropic"`
	FlagStimulant       int     `json:"flagStimulant"`
	FlagStimulantRaw    int     `json:"flagStimulantRaw"`
	ProcessFlagMA       string  `json:"processFlagMA"`
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
	JanUnitName          string `json:"janUnitName"`
	IsAdopted            bool   `json:"isAdopted,omitempty"`
}

type InventoryProductView struct {
	ProductMaster
	LastInventoryDate string `json:"lastInventoryDate"`
}

type Client struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type AggregationFilters struct {
	StartDate    string
	EndDate      string
	KanaName     string
	DrugTypes    []string
	DosageForm   string
	Coefficient  float64
	YjCode       string
	MovementOnly bool
	ShelfNumber  string
}

type ValuationFilters struct {
	Date                string
	KanaName            string
	UsageClassification string
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
	PackageKey             string              `json:"packageKey"`
	JanUnitName            string              `json:"janUnitName"`
	StartingBalance        interface{}         `json:"startingBalance"`
	Transactions           []LedgerTransaction `json:"transactions"`
	NetChange              float64             `json:"netChange"`
	EndingBalance          interface{}         `json:"endingBalance"`
	EffectiveEndingBalance float64             `json:"effectiveEndingBalance"`
	MaxUsage               float64             `json:"maxUsage"`
	ReorderPoint           float64             `json:"reorderPoint"`
	IsReorderNeeded        bool                `json:"isReorderNeeded"`
	Masters                []*ProductMaster    `json:"masters"`
	BaseReorderPoint       float64             `json:"baseReorderPoint"`
	PrecompoundedTotal     float64             `json:"precompoundedTotal"`
	DeliveryHistory        []TransactionRecord `json:"deliveryHistory,omitempty"`
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
	YjCode        string                  `json:"yjCode"`
	ProductName   string                  `json:"productName"`
	TotalStock    float64                 `json:"totalStock"`
	PackageGroups []DeadStockPackageGroup `json:"packageGroups"`
}

type DeadStockPackageGroup struct {
	PackageKey         string              `json:"packageKey"`
	TotalStock         float64             `json:"totalStock"`
	Products           []DeadStockProduct  `json:"products"`
	RecentTransactions []TransactionRecord `json:"recentTransactions,omitempty"`
}

type DeadStockProduct struct {
	ProductMaster
	CurrentStock  float64           `json:"currentStock"`
	SavedRecords  []DeadStockRecord `json:"savedRecords"`
	LastUsageDate string            `json:"lastUsageDate"`
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

type DeadStockFilters struct {
	StartDate        string
	EndDate          string
	ExcludeZeroStock bool
	Coefficient      float64
	KanaName         string
	DosageForm       string
	ShelfNumber      string
}

type PreCompoundingRecord struct {
	ID            int     `json:"id"`
	PatientNumber string  `json:"patientNumber"`
	ProductCode   string  `json:"productCode"`
	Quantity      float64 `json:"quantity"`
	CreatedAt     string  `json:"createdAt"`
}

type Wholesaler struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ▼▼▼【ここから修正】▼▼▼
type Backorder struct {
	ID                int            `json:"id"`
	OrderDate         string         `json:"orderDate"`
	YjCode            string         `json:"yjCode"`
	ProductName       string         `json:"productName"`
	PackageForm       string         `json:"packageForm"`
	JanPackInnerQty   float64        `json:"janPackInnerQty"`
	YjUnitName        string         `json:"yjUnitName"`
	OrderQuantity     float64        `json:"orderQuantity"`
	RemainingQuantity float64        `json:"remainingQuantity"`
	WholesalerCode    sql.NullString `json:"wholesalerCode,omitempty"`
	YjPackUnitQty     float64        `json:"yjPackUnitQty"`
	JanPackUnitQty    float64        `json:"janPackUnitQty"`
	JanUnitCode       int            `json:"janUnitCode"`
	// フロントエンドからの発注データ受け取り用フィールド
	YjQuantity float64 `json:"yjQuantity,omitempty"`
}

// ▲▲▲【修正ここまで】▲▲▲

type PriceUpdate struct {
	ProductCode      string  `json:"productCode"`
	NewPurchasePrice float64 `json:"newPrice"`
	NewSupplier      string  `json:"newWholesaler"`
}

type QuoteData struct {
	ProductMaster
	Quotes map[string]float64 `json:"quotes"`
}

type ValuationDetailRow struct {
	YjCode               string  `json:"yjCode"`
	ProductName          string  `json:"productName"`
	ProductCode          string  `json:"productCode"`
	PackageSpec          string  `json:"packageSpec"`
	Stock                float64 `json:"stock"`
	YjUnitName           string  `json:"yjUnitName"`
	PackageNhiPrice      float64 `json:"packageNhiPrice"`
	PackagePurchasePrice float64 `json:"packagePurchasePrice"`
	TotalNhiValue        float64 `json:"totalNhiValue"`
	TotalPurchaseValue   float64 `json:"totalPurchaseValue"`
	ShowAlert            bool    `json:"showAlert"`
}
