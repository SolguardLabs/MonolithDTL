package main

type Amount int64

type ProductKind string

const (
	ProductEscrow     ProductKind = "escrow"
	ProductVault      ProductKind = "vault"
	ProductRoute      ProductKind = "route"
	ProductOperator   ProductKind = "operator"
	ProductRedemption ProductKind = "redemption"
	ProductSystem     ProductKind = "system"
)

type AccountRole string

const (
	RoleUser      AccountRole = "user"
	RoleReserve   AccountRole = "reserve"
	RoleOperator  AccountRole = "operator"
	RoleTreasury  AccountRole = "treasury"
	RoleInsurance AccountRole = "insurance"
)

type Asset struct {
	ID            string `json:"id"`
	Symbol        string `json:"symbol"`
	Decimals      int    `json:"decimals"`
	RiskWeightBps int    `json:"risk_weight_bps"`
	HaircutBps    int    `json:"haircut_bps"`
	Enabled       bool   `json:"enabled"`
}

type Account struct {
	ID       string            `json:"id"`
	Owner    string            `json:"owner"`
	Role     AccountRole       `json:"role"`
	Balances map[string]Amount `json:"balances"`
	Frozen   bool              `json:"frozen"`
	Tags     []string          `json:"tags"`
}

type BalanceView struct {
	Account string `json:"account"`
	Owner   string `json:"owner"`
	Role    string `json:"role"`
	Asset   string `json:"asset"`
	Amount  Amount `json:"amount"`
	Frozen  bool   `json:"frozen"`
}

type LedgerEntry struct {
	ID        string      `json:"id"`
	Clock     int64       `json:"clock"`
	Product   ProductKind `json:"product"`
	ProductID string      `json:"product_id"`
	Account   string      `json:"account"`
	Asset     string      `json:"asset"`
	Debit     Amount      `json:"debit"`
	Credit    Amount      `json:"credit"`
	DebtDelta Amount      `json:"debt_delta"`
	Bucket    string      `json:"bucket"`
	Ref       string      `json:"ref"`
	Memo      string      `json:"memo"`
}

type GlobalDebtView struct {
	Account string `json:"account"`
	Asset   string `json:"asset"`
	Amount  Amount `json:"amount"`
}

type LocalSolvency struct {
	Product        ProductKind `json:"product"`
	ProductID      string      `json:"product_id"`
	Account        string      `json:"account"`
	Asset          string      `json:"asset"`
	Obligation     Amount      `json:"obligation"`
	Collateral     Amount      `json:"collateral"`
	Settled        Amount      `json:"settled"`
	Redeemed       Amount      `json:"redeemed"`
	Reserved       Amount      `json:"reserved"`
	Liquidated     Amount      `json:"liquidated"`
	State          string      `json:"state"`
	LastEvent      string      `json:"last_event"`
	LocalOnlyNotes []string    `json:"local_only_notes,omitempty"`
}

type LocalDebtView struct {
	Key         string      `json:"key"`
	Product     ProductKind `json:"product"`
	ProductID   string      `json:"product_id"`
	Account     string      `json:"account"`
	Asset       string      `json:"asset"`
	Outstanding Amount      `json:"outstanding"`
	Obligation  Amount      `json:"obligation"`
	Settled     Amount      `json:"settled"`
	Collateral  Amount      `json:"collateral"`
	State       string      `json:"state"`
}

type Event struct {
	ID        string                 `json:"id"`
	Clock     int64                  `json:"clock"`
	Kind      string                 `json:"kind"`
	Product   ProductKind            `json:"product"`
	ProductID string                 `json:"product_id"`
	Account   string                 `json:"account,omitempty"`
	Asset     string                 `json:"asset,omitempty"`
	Amount    Amount                 `json:"amount,omitempty"`
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Attrs     map[string]interface{} `json:"attrs,omitempty"`
}

type Operator struct {
	ID             string              `json:"id"`
	Account        string              `json:"account"`
	Name           string              `json:"name"`
	Active         bool                `json:"active"`
	Allowed        []ProductKind       `json:"allowed"`
	DailyLimit     Amount              `json:"daily_limit"`
	UsedToday      Amount              `json:"used_today"`
	RiskTier       string              `json:"risk_tier"`
	ProductCaps    map[ProductKind]int `json:"product_caps"`
	LastSettlement string              `json:"last_settlement"`
}

type OperatorView struct {
	ID          string   `json:"id"`
	Account     string   `json:"account"`
	Name        string   `json:"name"`
	Active      bool     `json:"active"`
	Allowed     []string `json:"allowed"`
	DailyLimit  Amount   `json:"daily_limit"`
	UsedToday   Amount   `json:"used_today"`
	RiskTier    string   `json:"risk_tier"`
	Utilization int      `json:"utilization_bps"`
}

type Escrow struct {
	ID          string `json:"id"`
	Depositor   string `json:"depositor"`
	Beneficiary string `json:"beneficiary"`
	Asset       string `json:"asset"`
	Amount      Amount `json:"amount"`
	Fee         Amount `json:"fee"`
	Released    Amount `json:"released"`
	Refunded    Amount `json:"refunded"`
	State       string `json:"state"`
	OperatorID  string `json:"operator_id"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

type Vault struct {
	ID          string `json:"id"`
	Asset       string `json:"asset"`
	Reserve     string `json:"reserve"`
	ShareAsset  string `json:"share_asset"`
	TotalShares Amount `json:"total_shares"`
	TotalAssets Amount `json:"total_assets"`
	OpenDebt    Amount `json:"open_debt"`
	FeeBps      int    `json:"fee_bps"`
	Paused      bool   `json:"paused"`
}

type VaultPosition struct {
	VaultID       string `json:"vault_id"`
	Account       string `json:"account"`
	Shares        Amount `json:"shares"`
	PrincipalDebt Amount `json:"principal_debt"`
	AccruedFees   Amount `json:"accrued_fees"`
	LastAction    string `json:"last_action"`
}

type RouteLeg struct {
	ID          string      `json:"id"`
	FromAccount string      `json:"from_account"`
	ToAccount   string      `json:"to_account"`
	Asset       string      `json:"asset"`
	Amount      Amount      `json:"amount"`
	Product     ProductKind `json:"product"`
	ProductID   string      `json:"product_id"`
	OperatorID  string      `json:"operator_id"`
	Status      string      `json:"status"`
}

type RoutePlan struct {
	ID             string     `json:"id"`
	SourceAccount  string     `json:"source_account"`
	Destination    string     `json:"destination"`
	Asset          string     `json:"asset"`
	Notional       Amount     `json:"notional"`
	MinOut         Amount     `json:"min_out"`
	SlippageBps    int        `json:"slippage_bps"`
	OperatorID     string     `json:"operator_id"`
	Legs           []RouteLeg `json:"legs"`
	Status         string     `json:"status"`
	SettledAmount  Amount     `json:"settled_amount"`
	RejectedReason string     `json:"rejected_reason,omitempty"`
	CreatedAt      int64      `json:"created_at"`
	SettledAt      int64      `json:"settled_at"`
}

type RedemptionRequest struct {
	ID            string      `json:"id"`
	Product       ProductKind `json:"product"`
	ProductID     string      `json:"product_id"`
	Account       string      `json:"account"`
	Asset         string      `json:"asset"`
	Shares        Amount      `json:"shares"`
	ExpectedOut   Amount      `json:"expected_out"`
	BlockedByDebt Amount      `json:"blocked_by_debt"`
	Status        string      `json:"status"`
	Reason        string      `json:"reason,omitempty"`
	CreatedAt     int64       `json:"created_at"`
	ProcessedAt   int64       `json:"processed_at"`
}

type ReconciliationLine struct {
	Account          string `json:"account"`
	Asset            string `json:"asset"`
	LocalOutstanding Amount `json:"local_outstanding"`
	GlobalDebt       Amount `json:"global_debt"`
	Drift            Amount `json:"drift"`
	Severity         string `json:"severity"`
}

type InvariantReport struct {
	LedgerNonNegative        bool `json:"ledger_non_negative"`
	LocalSolvencyNonNegative bool `json:"local_solvency_non_negative"`
	OperatorsWithinLimit     bool `json:"operators_within_limit"`
	RoutesConserved          bool `json:"routes_conserved"`
	ProductsHaveLocalState   bool `json:"products_have_local_state"`
	ObligationsReconciled    bool `json:"obligations_reconciled"`
	RedemptionsWithinPolicy  bool `json:"redemptions_within_policy"`
}

type RiskReport struct {
	ExecutedEscrows        int    `json:"executed_escrows"`
	ExecutedRoutes         int    `json:"executed_routes"`
	ProcessedRedemptions   int    `json:"processed_redemptions"`
	RejectedRedemptions    int    `json:"rejected_redemptions"`
	GlobalDebt             Amount `json:"global_debt"`
	LocalDebt              Amount `json:"local_debt"`
	ReconciliationVariance Amount `json:"reconciliation_variance"`
	ReconciliationAlert    bool   `json:"reconciliation_alert"`
	UnmatchedObligations   Amount `json:"unmatched_obligations"`
	ReserveVariance        Amount `json:"reserve_variance"`
}

type ScenarioReport struct {
	Protocol       string               `json:"protocol"`
	Scenario       string               `json:"scenario"`
	NetworkID      string               `json:"network_id"`
	Clock          int64                `json:"clock"`
	StateDigest    string               `json:"state_digest"`
	Assets         []Asset              `json:"assets"`
	Accounts       []BalanceView        `json:"accounts"`
	Operators      []OperatorView       `json:"operators"`
	Escrows        []Escrow             `json:"escrows"`
	Vaults         []Vault              `json:"vaults"`
	VaultPositions []VaultPosition      `json:"vault_positions"`
	Routes         []RoutePlan          `json:"routes"`
	Redemptions    []RedemptionRequest  `json:"redemptions"`
	GlobalDebt     []GlobalDebtView     `json:"global_debt"`
	LocalDebt      []LocalDebtView      `json:"local_debt"`
	Reconciliation []ReconciliationLine `json:"reconciliation"`
	Events         []Event              `json:"events"`
	LedgerEntries  []LedgerEntry        `json:"ledger_entries"`
	Invariants     InvariantReport      `json:"invariants"`
	Risk           RiskReport           `json:"risk"`
	Capital        CapitalAssessment    `json:"capital"`
	Notes          []string             `json:"notes"`
}
